package streaming

import (
	"encoding/json"
	"fmt"

	"github.com/atopos31/llmio/common/maputil"
	"github.com/atopos31/llmio/service/transform/shared"
)

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

	// 处理普通文本类型的 content block
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
		argsDelta := map[string]interface{}{
			"type":            "response.function_call_arguments.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    int(maputil.Float64(chunk, "index")),
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
	}

	outputTextDone := map[string]interface{}{
		"type":            "response.output_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    0,
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
		"output_index":    0,
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

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    0,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    "assistant",
			"content": contentItems,
			"status":  "completed",
		},
	}
	if err := writeRealtimeOrderedData(state, outputItemDone); err != nil {
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
			"output":     []interface{}{},
			"status":     status,
		},
	}

	// 添加 usage 信息
	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		usageMap := map[string]interface{}{
			"input_tokens":  int(maputil.Float64(usage, "input_tokens")),
			"output_tokens": int(maputil.Float64(usage, "output_tokens")),
			"total_tokens":  int(maputil.Float64(usage, "input_tokens") + maputil.Float64(usage, "output_tokens")),
		}
		// 添加 input_tokens_details
		if inputTokens := int(maputil.Float64(usage, "input_tokens")); inputTokens > 0 {
			usageMap["input_tokens_details"] = map[string]interface{}{
				"cached_tokens": 0,
			}
		}
		// 添加 output_tokens_details
		if outputTokens := int(maputil.Float64(usage, "output_tokens")); outputTokens > 0 {
			usageMap["output_tokens_details"] = map[string]interface{}{
				"reasoning_tokens": 0,
			}
		}
		responseCompleted["response"].(map[string]interface{})["usage"] = usageMap
	}

	return writeRealtimeOrderedData(state, responseCompleted)
}
