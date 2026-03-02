package transform

import "encoding/json"

func handleRealtimeOpenAIToAnthropic(state *realtimeStreamState, data string) error {
	// OpenAI → Anthropic 转换
	if data == "[DONE]" {
		messageStop := map[string]interface{}{"type": "message_stop"}
		return writeRealtimeEventJSONData(state, "message_stop", messageStop)
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil
	}
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil
	}

	// 处理角色信息（第一个chunk）
	if role := getString(delta, "role"); role != "" {
		_ = role
		messageStart := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":      getString(chunk, "id"),
				"type":    "message",
				"role":    "assistant",
				"content": []interface{}{},
				"model":   getString(chunk, "model"),
				"usage": map[string]interface{}{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}
		if err := writeRealtimeEventJSONData(state, "message_start", messageStart); err != nil {
			return err
		}

		blockStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		if err := writeRealtimeEventJSONData(state, "content_block_start", blockStart); err != nil {
			return err
		}
	}

	// 处理内容
	if content := getString(delta, "content"); content != "" {
		contentDelta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": content,
			},
		}
		if err := writeRealtimeEventJSONData(state, "content_block_delta", contentDelta); err != nil {
			return err
		}
	}

	// 处理结束
	finishReason := getString(choice, "finish_reason")
	if finishReason == "" {
		return nil
	}

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}

	stopReason := "end_turn"
	if finishReason == "tool_calls" {
		stopReason = "tool_use"
	} else if finishReason == "length" {
		stopReason = "max_tokens"
	}

	messageDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
	}

	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		messageDelta["usage"] = map[string]interface{}{
			"input_tokens":  int(getFloat(usage, "prompt_tokens")),
			"output_tokens": int(getFloat(usage, "completion_tokens")),
		}
	}

	if err := writeRealtimeEventJSONData(state, "message_delta", messageDelta); err != nil {
		return err
	}

	messageStop := map[string]interface{}{"type": "message_stop"}
	return writeRealtimeEventJSONData(state, "message_stop", messageStop)
}
