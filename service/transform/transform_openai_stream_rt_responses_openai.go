package transform

import (
	"encoding/json"
	"fmt"
	"time"
)

func handleRealtimeResponsesToOpenAI(state *realtimeStreamState, data string) error {
	// [DONE] 被忽略，因为 response.completed 已发送 [DONE]
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
		ensureOpenAIStreamMetaFromResponses(state, chunk)
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
		ensureOpenAIStreamMetaFromResponses(state, chunk)
		// 处理工具调用开始（发送 id/type/name）
		item, ok := chunk["item"].(map[string]interface{})
		if !ok {
			return nil
		}
		if getString(item, "type") != "function_call" {
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
						"tool_calls": []map[string]interface{}{
							{
								"index": int(getFloat(chunk, "output_index")),
								"id":    getString(item, "id"),
								"type":  "function",
								"function": map[string]interface{}{
									"name":      getString(item, "name"),
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
		ensureOpenAIStreamMetaFromResponses(state, chunk)
		// 发送文本增量
		delta := getString(chunk, "delta")
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
		ensureOpenAIStreamMetaFromResponses(state, chunk)
		// 发送 reasoning 增量（Extended Thinking）
		delta := getString(chunk, "delta")
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
		ensureOpenAIStreamMetaFromResponses(state, chunk)
		// 发送工具调用参数增量
		delta := getString(chunk, "delta")
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
						"tool_calls": []map[string]interface{}{
							{
								"index": int(getFloat(chunk, "output_index")),
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

	case "response.completed":
		return handleResponsesToOpenAICompleted(state, chunk)
	default:
		return nil
	}
}

func handleResponsesToOpenAICompleted(state *realtimeStreamState, chunk map[string]interface{}) error {
	ensureOpenAIStreamMetaFromResponses(state, chunk)

	// 发送结束块
	finishReason := "stop"
	if response, ok := chunk["response"].(map[string]interface{}); ok {
		status := getString(response, "status")
		if status == "incomplete" {
			finishReason = "length"
		} else if status == "failed" {
			finishReason = "stop"
		}

		// 检查是否有工具调用（通过 output 判断）
		if output, ok := response["output"].([]interface{}); ok && len(output) > 0 {
			for _, item := range output {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if getString(itemMap, "type") == "function_call" {
						finishReason = "tool_calls"
						break
					}
				}
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
	if response, ok := chunk["response"].(map[string]interface{}); ok {
		if usage, ok := response["usage"].(map[string]interface{}); ok {
			finalChunk["usage"] = map[string]interface{}{
				"prompt_tokens":     int(getFloat(usage, "input_tokens")),
				"completion_tokens": int(getFloat(usage, "output_tokens")),
				"total_tokens":      int(getFloat(usage, "total_tokens")),
			}
		}
	}

	if err := writeRealtimeJSONData(state, finalChunk); err != nil {
		return err
	}

	// 发送 [DONE]
	return writeRealtimeData(state, "[DONE]")
}

func ensureOpenAIStreamMetaFromResponses(state *realtimeStreamState, chunk map[string]interface{}) {
	if state.openAICreated == 0 {
		state.openAICreated = time.Now().Unix()
	}
	if state.openAIModel == "" {
		state.openAIModel = "responses-api"
	}
	if state.openAIID != "" {
		return
	}

	if resp, ok := chunk["response"].(map[string]interface{}); ok {
		if id := getString(resp, "id"); id != "" {
			state.openAIID = "chatcmpl-" + id
		}
		if model := getString(resp, "model"); model != "" {
			state.openAIModel = model
		}
		if createdAt := int64(getFloat(resp, "created_at")); createdAt > 0 {
			state.openAICreated = createdAt
		}
	}

	if state.openAIID == "" {
		if responseID := getString(chunk, "response_id"); responseID != "" {
			state.openAIID = "chatcmpl-" + responseID
		} else if responseID := getNestedString(chunk, "response.id"); responseID != "" {
			state.openAIID = "chatcmpl-" + responseID
		}
	}

	if state.openAIID == "" {
		state.openAIID = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
}
