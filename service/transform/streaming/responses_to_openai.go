package streaming

import (
	"encoding/json"

	"github.com/atopos31/llmio/service/responses"
)

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

	// 添加 usage 信息
	if ev.Response != nil && ev.Response.Usage != nil {
		finalChunk["usage"] = map[string]interface{}{
			"prompt_tokens":     int(ev.Response.Usage.InputTokens),
			"completion_tokens": int(ev.Response.Usage.OutputTokens),
			"total_tokens":      int(ev.Response.Usage.TotalTokens),
		}
	}

	if err := writeRealtimeJSONData(state, finalChunk); err != nil {
		return err
	}

	// 发送 [DONE]
	return writeRealtimeData(state, "[DONE]")
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
