package streaming

import (
	"encoding/json"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/responses"
)

func init() {
	RegisterRealtimeRoute(consts.FormatOpenAIResponses, consts.FormatOpenAIChat, handleRealtimeResponsesToOpenAI)
}

func handleRealtimeResponsesToOpenAI(state *realtimeStreamState, data string) error {
	// [DONE] 被忽略，因为 response.completed 已发送 [DONE]
	if data == "[DONE]" {
		return nil
	}

	var ev responses.ResponsesStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	eventType := state.currentEvent
	if eventType == "" {
		eventType = ev.Type
	}

	switch eventType {
	case "response.created":
		ensureOpenAIStreamMetaFromResponsesEvent(state, &ev)
		openAIChunk := map[string]interface{}{
			"id":      state.openAIID,
			"object":  "chat.completion.chunk",
			"created": state.openAICreated,
			"model":   state.openAIModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"role": "assistant",
					},
					"finish_reason": nil,
				},
			},
		}
		return writeRealtimeJSONData(state, openAIChunk)

	case "response.output_item.added":
		ensureOpenAIStreamMetaFromResponsesEvent(state, &ev)
		// 处理工具调用开始（发送 id/type/name）
		if ev.Item == nil {
			return nil
		}
		if ev.Item.Type != "function_call" {
			return nil
		}

		toolCallIndex := getOrAllocOpenAIToolCallIndex(state, ev.OutputIndex)
		openAIChunk := map[string]interface{}{
			"id":      state.openAIID,
			"object":  "chat.completion.chunk",
			"created": state.openAICreated,
			"model":   state.openAIModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{
							{
								"index": toolCallIndex,
								"id":    ev.Item.ID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      derefString(ev.Item.Name),
									"arguments": "",
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		return writeRealtimeJSONData(state, openAIChunk)

	case "response.output_text.delta":
		ensureOpenAIStreamMetaFromResponsesEvent(state, &ev)
		// 发送文本增量
		delta := ev.Delta
		if delta == "" {
			return nil
		}

		openAIChunk := map[string]interface{}{
			"id":      state.openAIID,
			"object":  "chat.completion.chunk",
			"created": state.openAICreated,
			"model":   state.openAIModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": delta,
					},
					"finish_reason": nil,
				},
			},
		}
		return writeRealtimeJSONData(state, openAIChunk)

	case "response.reasoning_summary_text.delta":
		ensureOpenAIStreamMetaFromResponsesEvent(state, &ev)
		// 发送 reasoning 增量（Extended Thinking）
		delta := ev.Delta
		if delta == "" {
			return nil
		}

		openAIChunk := map[string]interface{}{
			"id":      state.openAIID,
			"object":  "chat.completion.chunk",
			"created": state.openAICreated,
			"model":   state.openAIModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"reasoning_content": delta,
					},
					"finish_reason": nil,
				},
			},
		}
		return writeRealtimeJSONData(state, openAIChunk)

	case "response.function_call_arguments.delta":
		ensureOpenAIStreamMetaFromResponsesEvent(state, &ev)
		// 发送工具调用参数增量
		delta := ev.Delta
		if delta == "" {
			return nil
		}

		toolCallIndex := getOpenAIToolCallIndex(state, ev.OutputIndex)
		openAIChunk := map[string]interface{}{
			"id":      state.openAIID,
			"object":  "chat.completion.chunk",
			"created": state.openAICreated,
			"model":   state.openAIModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{
							{
								"index": toolCallIndex,
								"function": map[string]interface{}{
									"arguments": delta,
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		return writeRealtimeJSONData(state, openAIChunk)

	case "response.content_part.added":
		// OpenAI Chat Completions streaming does not support multimodal parts.
		// Best-effort: ignore non-text parts.
		return nil

	case "response.completed":
		return handleResponsesToOpenAICompletedEvent(state, &ev)
	default:
		return nil
	}
}

func handleResponsesToOpenAICompletedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	ensureOpenAIStreamMetaFromResponsesEvent(state, ev)

	// 发送结束块
	finishReason := "stop"
	if ev.Response != nil {
		if ev.Response.Status != nil {
			switch *ev.Response.Status {
			case "incomplete":
				finishReason = "length"
			case "failed":
				finishReason = "stop"
			}
		}

		for _, item := range ev.Response.Output {
			if item.Type == "function_call" {
				finishReason = "tool_calls"
				break
			}
		}
	}

	finalChunk := map[string]interface{}{
		"id":      state.openAIID,
		"object":  "chat.completion.chunk",
		"created": state.openAICreated,
		"model":   state.openAIModel,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": finishReason,
			},
		},
	}

	if err := writeRealtimeJSONData(state, finalChunk); err != nil {
		return err
	}

	// usage 独立成尾包，不与 finish_reason 同包：OpenAI 流式规范里
	// stream_options.include_usage 的 usage 是在 finish_reason **之后**单独发一个
	// choices:[] 的包。本仓入站侧已按此规范实现（realtime.go 的 pendingUsage 为此
	// 延后 response.completed），出站若塞进 finish 包，等于对同一协议事实持两套
	// 相反假设，按规范只在尾包找 usage 的下游会读到 0。
	if ev.Response != nil && ev.Response.Usage != nil {
		u := ev.Response.Usage
		// 上游 openai-res 原始 usage 旁路交给落库侧（本跳读上游时才持有 sideChannel）。
		// 全零快照由 SetUpstreamUsage 判据丢弃，故此处无条件交，判空交给侧信道收口。
		captureUpstreamUsageResponses(state, u)

		// 全零 usage 不单独发尾包：拆包后尾包独占一个 chunk，下游会把「尾包存在」
		// 读成「上游明确报告了 token 数」。与仓内既有口径一致——buildResponsesUsage
		// 空则不写、侧信道全零丢弃、processer 要求 total!=0——全零不是有效观测。
		if responsesUsageHasTokens(u) {
			usageChunk := map[string]interface{}{
				"id":      state.openAIID,
				"object":  "chat.completion.chunk",
				"created": state.openAICreated,
				"model":   state.openAIModel,
				"choices": []map[string]interface{}{},
				// 与另两个写出点共用装配实现：total 回退、零值明细判据只有一份。
				"usage": openAIUsageFromModel(usageFromResponses(u)),
			}
			if err := writeRealtimeJSONData(state, usageChunk); err != nil {
				return err
			}
		}
	}

	// 发送 [DONE]
	return writeRealtimeData(state, "[DONE]")
}

// responsesUsageHasTokens 判定 usage 是否含有效 token，与 models.Usage.HasTokens 同口径：
// 只看总量三字段，不看 cache / reasoning 明细——只有 cache 命中时总量仍为 0
// （total 口径不含 Anthropic cache token），保留也拿不到计费值。调用方保证 u != nil。
func responsesUsageHasTokens(u *responses.ResponsesUsage) bool {
	return u.InputTokens > 0 || u.OutputTokens > 0 || u.TotalTokens > 0
}

func ensureOpenAIStreamMetaFromResponsesEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) {
	if state.openAIModel == "" {
		state.openAIModel = "responses-api"
	}
	if state.openAIID != "" {
		return
	}

	if ev != nil && ev.Response != nil {
		if ev.Response.ID != "" {
			state.openAIID = "chatcmpl-" + ev.Response.ID
		}
		if ev.Response.Model != "" {
			state.openAIModel = ev.Response.Model
		}
		if ev.Response.CreatedAt > 0 {
			state.openAICreated = ev.Response.CreatedAt
		}
	}

	if state.openAIID == "" {
		if ev != nil && ev.ResponseID != "" {
			state.openAIID = "chatcmpl-" + ev.ResponseID
		}
	}

	if state.openAIID == "" {
		state.openAIID = "chatcmpl-unknown"
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getOrAllocOpenAIToolCallIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToOpenAIToolCallIndex == nil {
		state.responsesOutputIndexToOpenAIToolCallIndex = map[int]int{}
	}
	if idx, ok := state.responsesOutputIndexToOpenAIToolCallIndex[outputIndex]; ok {
		return idx
	}
	idx := state.openAIToolCallNextIndex
	state.openAIToolCallNextIndex++
	state.responsesOutputIndexToOpenAIToolCallIndex[outputIndex] = idx
	return idx
}

func getOpenAIToolCallIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToOpenAIToolCallIndex == nil {
		return outputIndex
	}
	if idx, ok := state.responsesOutputIndexToOpenAIToolCallIndex[outputIndex]; ok {
		return idx
	}
	return outputIndex
}
