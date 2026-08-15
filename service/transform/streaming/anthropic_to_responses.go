package streaming

import (
	"encoding/json"
	"fmt"

	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/service/transform/shared"
)

func init() {
	RegisterRealtimeRoute("anthropic", "openai-res", handleRealtimeAnthropicToResponses)
}

func handleRealtimeAnthropicToResponses(state *realtimeStreamState, data string) error {
	// Anthropic 不发送 [DONE]，忽略
	if data == "[DONE]" {
		return nil
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	eventType := state.currentEvent
	if eventType == "" {
		eventType = maputil.String(chunk, "type")
	}

	switch eventType {
	case "message_start":
		return handleAnthropicToResponsesMessageStart(state, chunk)
	case "content_block_start":
		return handleAnthropicToResponsesContentBlockStart(state, chunk)
	case "content_block_delta":
		return handleAnthropicToResponsesContentBlockDelta(state, chunk)
	case "message_delta":
		return handleAnthropicToResponsesMessageDelta(state, chunk)
	case "message_stop":
		// Responses API 不需要单独的 stop 事件
		return nil
	default:
		return nil
	}
}

func handleAnthropicToResponsesMessageStart(state *realtimeStreamState, chunk map[string]interface{}) error {
	// 获取 message ID 并生成 item ID
	state.responseID = shared.GetNestedString(chunk, "message.id")
	state.itemID = fmt.Sprintf("msg_%s", state.responseID)
	createdAt := int(shared.GetNestedFloat(chunk, "message.created_at"))

	// input 侧 token 只在 message_start 出现（旧版上游的 message_delta 不重复带），
	// 缓存下来供 message_delta 构建 response.completed.usage 时合并。
	if message, ok := chunk["message"].(map[string]interface{}); ok {
		if usage, ok := message["usage"].(map[string]interface{}); ok {
			state.anthropicStartUsage = usage
			// 同时立刻交一份给侧信道：上游若在 message_start 之后异常结束
			// （连接中断 / error 事件 / 无 delta 的空回复），message_delta 分支
			// 不会执行，input 侧真值（缓存命中数是计费大头）就永久丢了。
			// 后续 message_delta 交的是含 input 侧的合并快照，是本次的超集，
			// 「最后观测胜出」不会造成降级。
			captureUpstreamUsageMap(state, usage)
		}
	}

	// 发送 response.created 事件
	responseCreated := map[string]interface{}{
		"type":            "response.created",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": createdAt,
			"output":     []interface{}{},
			"status":     "in_progress",
		},
	}
	if err := writeRealtimeOrderedData(state, responseCreated); err != nil {
		return err
	}

	// 发送 response.in_progress 事件
	responseInProgress := map[string]interface{}{
		"type":            "response.in_progress",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": createdAt,
			"output":     []interface{}{},
			"status":     "in_progress",
		},
	}
	return writeRealtimeOrderedData(state, responseInProgress)
}

func handleAnthropicToResponsesContentBlockStart(state *realtimeStreamState, chunk map[string]interface{}) error {
	contentBlock, ok := chunk["content_block"].(map[string]interface{})
	if !ok {
		return nil
	}
	blockType := maputil.String(contentBlock, "type")
	blockIndex := int(maputil.Float64(chunk, "index"))

	if blockType == "thinking" {
		// 处理 thinking 类型的 content block（Extended Thinking）
		if state.hasReasoningItem {
			return nil
		}
		state.hasReasoningItem = true
		state.reasoningItemID = fmt.Sprintf("reasoning_%s", state.responseID)
		state.reasoningOutputIndex = blockIndex

		reasoningItemAdded := map[string]interface{}{
			"type":            "response.output_item.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item": map[string]interface{}{
				"id":      state.reasoningItemID,
				"type":    "reasoning",
				"status":  "in_progress",
				"summary": []interface{}{},
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningItemAdded); err != nil {
			return err
		}

		summaryPartAdded := map[string]interface{}{
			"type":            "response.reasoning_summary_part.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"part": map[string]interface{}{
				"type": "summary_text",
			},
		}
		if err := writeRealtimeOrderedData(state, summaryPartAdded); err != nil {
			return err
		}

		state.accumulatedReasoning = ""
		return nil
	}

	if blockType == "tool_use" {
		// 工具调用块：必须映射成 function_call item 并透传 id/name，
		// 落进下面的通用分支会被包成 type="message"，id 与 name 一起丢掉。
		toolID := maputil.String(contentBlock, "id")
		item := map[string]interface{}{
			"type":      "function_call",
			"id":        toolID,
			"call_id":   toolID,
			"name":      maputil.String(contentBlock, "name"),
			"arguments": "",
		}
		state.responsesOutput.set(blockIndex, item)

		toolItemAdded := map[string]interface{}{
			"type":            "response.output_item.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    blockIndex,
			"item":            item,
		}
		return writeRealtimeOrderedData(state, toolItemAdded)
	}

	// 处理普通文本类型的 content block
	if !state.hasMessageOutputIndex {
		state.messageOutputIndex = blockIndex
		state.hasMessageOutputIndex = true
	}

	itemAdded := map[string]interface{}{
		"type":            "response.output_item.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    blockIndex,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"status":  "in_progress",
		},
	}
	if err := writeRealtimeOrderedData(state, itemAdded); err != nil {
		return err
	}

	contentPartAdded := map[string]interface{}{
		"type":            "response.content_part.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    blockIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
		},
	}
	if err := writeRealtimeOrderedData(state, contentPartAdded); err != nil {
		return err
	}

	state.accumulatedText = ""
	return nil
}

func handleAnthropicToResponsesContentBlockDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	delta, ok := chunk["delta"].(map[string]interface{})
	if !ok {
		return nil
	}
	deltaType := maputil.String(delta, "type")

	if deltaType == "thinking_delta" {
		text := maputil.String(delta, "thinking")
		if text == "" {
			return nil
		}
		state.accumulatedReasoning += text
		reasoningDelta := map[string]interface{}{
			"type":            "response.reasoning_summary_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"delta":           text,
		}
		return writeRealtimeOrderedData(state, reasoningDelta)
	}

	if deltaType == "text_delta" {
		text := maputil.String(delta, "text")
		if text == "" {
			return nil
		}
		state.accumulatedText += text
		textDelta := map[string]interface{}{
			"type":            "response.output_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    int(maputil.Float64(chunk, "index")),
			"item_id":         state.itemID,
			"content_index":   0,
			"delta":           text,
		}
		return writeRealtimeOrderedData(state, textDelta)
	}

	if deltaType == "input_json_delta" {
		partialJSON := maputil.String(delta, "partial_json")
		if partialJSON == "" {
			return nil
		}
		blockIndex := int(maputil.Float64(chunk, "index"))
		state.responsesOutput.appendArguments(blockIndex, partialJSON)

		argsDelta := map[string]interface{}{
			"type":            "response.function_call_arguments.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    blockIndex,
			"delta":           partialJSON,
		}
		return writeRealtimeOrderedData(state, argsDelta)
	}

	return nil
}

func handleAnthropicToResponsesMessageDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	// 如果有 reasoning 内容，先发送 reasoning 完成事件
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		reasoningTextDone := map[string]interface{}{
			"type":            "response.reasoning_summary_text.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"text":            state.accumulatedReasoning,
		}
		if err := writeRealtimeOrderedData(state, reasoningTextDone); err != nil {
			return err
		}

		reasoningPartDone := map[string]interface{}{
			"type":            "response.reasoning_summary_part.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"part": map[string]interface{}{
				"type": "summary_text",
				"text": state.accumulatedReasoning,
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningPartDone); err != nil {
			return err
		}

		reasoningItemDone := map[string]interface{}{
			"type":            "response.output_item.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item": map[string]interface{}{
				"id":   state.reasoningItemID,
				"type": "reasoning",
				"summary": []map[string]interface{}{
					{
						"type": "summary_text",
						"text": state.accumulatedReasoning,
					},
				},
				"status": "completed",
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningItemDone); err != nil {
			return err
		}

		state.responsesOutput.set(state.reasoningOutputIndex, map[string]interface{}{
			"id":   state.reasoningItemID,
			"type": "reasoning",
			"summary": []map[string]interface{}{
				{
					"type": "summary_text",
					"text": state.accumulatedReasoning,
				},
			},
			"status": "completed",
		})
	}

	// 只有真的出现过文本块才发 message 类型 done 事件：纯 tool_use 流不应再带
	// output_text.done / content_part.done / output_item.done(message)，否则
	// messageOutputIndex=0 会与 blockIndex=0 的 function_call 撞车，
	// 且事件流本身冗余。reasoning 路径在上面已独立发完，与这里互不影响。
	if err := emitMessageStreamFinalization(state); err != nil {
		return err
	}

	status := "completed"
	if delta, ok := chunk["delta"].(map[string]interface{}); ok {
		if reason := maputil.String(delta, "stop_reason"); reason != "" && reason == "max_tokens" {
			status = "incomplete"
		}
	}

	responseCompleted := map[string]interface{}{
		"type":            "response.completed",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": 0,
			// 必须是真实产出：下游靠扫这个数组里有没有 function_call 判定 finish_reason。
			"output": state.responsesOutput.snapshot(),
			"status": status,
		},
	}

	// 合并 usage：output 侧来自本 message_delta，input 侧优先取 message_delta，
	// 缺失时回退到 message_start 缓存（旧版上游只在 message_start 给 input_tokens）。
	deltaUsage, _ := chunk["usage"].(map[string]interface{})
	if deltaUsage != nil || state.anthropicStartUsage != nil {
		pick := func(key string) float64 {
			if deltaUsage != nil {
				if v := maputil.Float64(deltaUsage, key); v > 0 {
					return v
				}
			}
			if state.anthropicStartUsage != nil {
				return maputil.Float64(state.anthropicStartUsage, key)
			}
			return 0
		}

		inputTokens := pick("input_tokens")
		outputTokens := pick("output_tokens")
		cacheRead := pick("cache_read_input_tokens")

		// 合并后的 anthropic 原生 usage 交给侧信道。这里交的是含 input 侧的
		// 完整快照，是 message_start 那次的超集，故覆盖不会降级。
		captureUpstreamUsageMap(state, map[string]interface{}{
			"input_tokens":            inputTokens,
			"output_tokens":           outputTokens,
			"cache_read_input_tokens": cacheRead,
		})

		usageMap := map[string]interface{}{
			"input_tokens":  int(inputTokens),
			"output_tokens": int(outputTokens),
			// Anthropic 的 cache_read / cache_creation 与 input_tokens 并列计数，
			// 但落库口径统一为 input+output（见 process.go），此处 total 保持同口径。
			"total_tokens": int(inputTokens + outputTokens),
		}
		// 只有 cache_read 有真值才写 details：它是唯一能映射到 cached_tokens 的量。
		// 不能把 cache_creation 也当门禁——上游首次写缓存时 cache_creation>0 而
		// cache_read=0，那样会输出 cached_tokens:0，正是下面注释要避免的情况。
		// cache_creation（缓存写入数）本网关不统计，与 total 口径一致地丢弃。
		if cacheRead > 0 {
			usageMap["input_tokens_details"] = map[string]interface{}{
				"cached_tokens": int(cacheRead),
			}
		}
		// Anthropic 协议无 reasoning token（thinking 计入 output_tokens），
		// 故不输出 output_tokens_details（写 reasoning_tokens:0 会让下游误以为上游明确报告了 0）。
		responseCompleted["response"].(map[string]interface{})["usage"] = usageMap
	}

	return writeRealtimeOrderedData(state, responseCompleted)
}

// emitMessageStreamFinalization 发出 message 类型 content 块的收尾事件集：
// output_text.done → content_part.done → output_item.done(message)。
// 仅在 hasMessageOutputIndex 为真时执行，纯 tool_use 流不会进入此分支。
// 抽出的理由：原逻辑嵌在 handleAnthropicToResponsesMessageDelta 中导致该函数 80→130 行膨胀；
// 独立函数后 reasoning 路径与 message 路径边界更清晰（SRP），便于复用与单测。
func emitMessageStreamFinalization(state *realtimeStreamState) error {
	if !state.hasMessageOutputIndex {
		return nil
	}
	msgIndex := state.messageOutputIndex

	outputTextDone := map[string]interface{}{
		"type":            "response.output_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"text":            state.accumulatedText,
	}
	if err := writeRealtimeOrderedData(state, outputTextDone); err != nil {
		return err
	}

	contentPartDone := map[string]interface{}{
		"type":            "response.content_part.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	if err := writeRealtimeOrderedData(state, contentPartDone); err != nil {
		return err
	}

	contentItems := []map[string]interface{}{
		{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	// 如果有 reasoning，添加到 content 数组前面
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		contentItems = append([]map[string]interface{}{
			{
				"type": "reasoning",
				"id":   state.reasoningItemID,
				"summary": []map[string]interface{}{
					{
						"type": "summary_text",
						"text": state.accumulatedReasoning,
					},
				},
			},
		}, contentItems...)
	}

	messageItem := map[string]interface{}{
		"id":      state.itemID,
		"type":    "message",
		"role":    "assistant",
		"content": contentItems,
		"status":  "completed",
	}
	// 只有真的出现过文本块才记入 output：否则纯 tool_use 的流会把
	// index 0 上的 function_call 覆盖掉。
	state.responsesOutput.set(msgIndex, messageItem)

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item":            messageItem,
	}
	return writeRealtimeOrderedData(state, outputItemDone)
}
