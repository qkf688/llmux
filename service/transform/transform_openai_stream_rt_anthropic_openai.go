package transform

import (
	"encoding/json"
	"fmt"
	"time"
)

func handleRealtimeAnthropicToOpenAI(state *realtimeStreamState, data string) error {
	if data == "[DONE]" {
		return writeRealtimeData(state, "[DONE]")
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	// 优先使用 event 行的事件类型,如果没有则从 JSON 中获取
	eventType := state.currentEvent
	if eventType == "" {
		eventType = getString(chunk, "type")
	}

	switch eventType {
	case "message_start", "ping":
		// 忽略这些事件
		return nil

	case "content_block_start":
		// 处理工具调用开始
		contentBlock, ok := chunk["content_block"].(map[string]interface{})
		if !ok {
			return nil
		}
		if getString(contentBlock, "type") != "tool_use" {
			return nil
		}

		openAIChunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "claude",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"index": getFloat(chunk, "index"),
								"id":    getString(contentBlock, "id"),
								"type":  "function",
								"function": map[string]interface{}{
									"name":      getString(contentBlock, "name"),
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

	case "content_block_delta":
		// 提取文本内容或工具调用参数并转换为 OpenAI 格式
		delta, ok := chunk["delta"].(map[string]interface{})
		if !ok {
			return nil
		}
		deltaType := getString(delta, "type")
		if deltaType == "text_delta" {
			text := getString(delta, "text")
			if text == "" {
				return nil
			}
			openAIChunk := map[string]interface{}{
				"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "claude",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": text,
						},
						"finish_reason": nil,
					},
				},
			}
			return writeRealtimeJSONData(state, openAIChunk)
		}
		if deltaType == "input_json_delta" {
			partialJSON := getString(delta, "partial_json")
			if partialJSON == "" {
				return nil
			}
			openAIChunk := map[string]interface{}{
				"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "claude",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"index": getFloat(chunk, "index"),
									"function": map[string]interface{}{
										"arguments": partialJSON,
									},
								},
							},
						},
						"finish_reason": nil,
					},
				},
			}
			return writeRealtimeJSONData(state, openAIChunk)
		}
		return nil

	case "content_block_stop":
		// 忽略内容块停止事件
		return nil

	case "message_delta":
		// 发送结束块
		stopReason := "stop"
		if delta, ok := chunk["delta"].(map[string]interface{}); ok {
			if reason := getString(delta, "stop_reason"); reason != "" {
				if reason == "end_turn" {
					stopReason = "stop"
				} else if reason == "tool_use" {
					stopReason = "tool_calls"
				}
			}
		}

		finalChunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "claude",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": stopReason,
				},
			},
		}

		// 添加 usage 信息
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			finalChunk["usage"] = map[string]interface{}{
				"prompt_tokens":     int(getFloat(usage, "input_tokens")),
				"completion_tokens": int(getFloat(usage, "output_tokens")),
				"total_tokens":      int(getFloat(usage, "input_tokens") + getFloat(usage, "output_tokens")),
			}
		}
		return writeRealtimeJSONData(state, finalChunk)

	case "message_stop":
		// 发送 [DONE]
		return writeRealtimeData(state, "[DONE]")
	}

	return nil
}
