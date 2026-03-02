package transform

import "encoding/json"

func handleRealtimeResponsesToAnthropic(state *realtimeStreamState, data string) error {
	// [DONE] 被忽略，因为 response.completed 已发送 message_stop
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
		eventType = getString(chunk, "type")
	}

	switch eventType {
	case "response.created":
		return handleResponsesToAnthropicCreated(state, chunk)
	case "response.output_item.added":
		return handleResponsesToAnthropicOutputItemAdded(state, chunk)
	case "response.output_text.delta":
		return handleResponsesToAnthropicOutputTextDelta(state, chunk)
	case "response.reasoning_summary_text.delta":
		return handleResponsesToAnthropicReasoningDelta(state, chunk)
	case "response.function_call_arguments.delta":
		return handleResponsesToAnthropicFunctionArgsDelta(state, chunk)
	case "response.completed":
		return handleResponsesToAnthropicCompleted(state, chunk)
	default:
		return nil
	}
}

func handleResponsesToAnthropicCreated(state *realtimeStreamState, chunk map[string]interface{}) error {
	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      getNestedString(chunk, "response.id"),
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   "responses-api",
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	return writeRealtimeEventJSONData(state, "message_start", messageStart)
}

func handleResponsesToAnthropicOutputItemAdded(state *realtimeStreamState, chunk map[string]interface{}) error {
	item, ok := chunk["item"].(map[string]interface{})
	if !ok {
		return nil
	}
	itemType := getString(item, "type")

	blockStart := map[string]interface{}{
		"type":  "content_block_start",
		"index": int(getFloat(chunk, "output_index")),
		"content_block": map[string]interface{}{
			"type": itemType,
		},
	}

	// 映射 Responses 类型 → Anthropic 类型
	if itemType == "function_call" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "tool_use",
			"id":   getString(item, "id"),
			"name": getString(item, "name"),
		}
	} else if itemType == "reasoning" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "thinking",
		}
	} else if itemType == "output_text" || itemType == "text" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "text",
			"text": "",
		}
	}

	return writeRealtimeEventJSONData(state, "content_block_start", blockStart)
}

func handleResponsesToAnthropicOutputTextDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	delta := getString(chunk, "delta")
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": int(getFloat(chunk, "output_index")),
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicReasoningDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	delta := getString(chunk, "delta")
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": int(getFloat(chunk, "output_index")),
		"delta": map[string]interface{}{
			"type":     "thinking_delta",
			"thinking": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicFunctionArgsDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	delta := getString(chunk, "delta")
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": int(getFloat(chunk, "output_index")),
		"delta": map[string]interface{}{
			"type":         "input_json_delta",
			"partial_json": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicCompleted(state *realtimeStreamState, chunk map[string]interface{}) error {
	stopReason := "end_turn"
	blockStopIndex := 0

	if response, ok := chunk["response"].(map[string]interface{}); ok {
		if getString(response, "status") == "incomplete" {
			stopReason = "max_tokens"
		}
		// 检查是否有工具调用（通过 output 判断）
		if output, ok := response["output"].([]interface{}); ok && len(output) > 0 {
			blockStopIndex = len(output) - 1
			for _, rawItem := range output {
				itemMap, ok := rawItem.(map[string]interface{})
				if !ok {
					continue
				}
				if getString(itemMap, "type") == "function_call" {
					stopReason = "tool_use"
					break
				}
			}
		}
	}

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": blockStopIndex,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}

	messageDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
	}

	// 添加 usage 信息
	if response, ok := chunk["response"].(map[string]interface{}); ok {
		if usage, ok := response["usage"].(map[string]interface{}); ok {
			messageDelta["usage"] = map[string]interface{}{
				"input_tokens":  int(getFloat(usage, "input_tokens")),
				"output_tokens": int(getFloat(usage, "output_tokens")),
			}
		}
	}

	if err := writeRealtimeEventJSONData(state, "message_delta", messageDelta); err != nil {
		return err
	}

	messageStop := map[string]interface{}{"type": "message_stop"}
	return writeRealtimeEventJSONData(state, "message_stop", messageStop)
}
