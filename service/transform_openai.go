package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atopos31/llmio/models"
)

// TransformOpenAIToUnified 将 OpenAI 格式转换为统一格式
func TransformOpenAIToUnified(rawBody []byte) (*UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &UnifiedRequest{
		Model:  getString(req, "model"),
		Stream: getBool(req, "stream"),
	}

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		unified.MaxTokens = int(maxTokens)
	}
	if temp, ok := req["temperature"].(float64); ok {
		unified.Temperature = &temp
	}
	if topP, ok := req["top_p"].(float64); ok {
		unified.TopP = &topP
	}

	// 转换消息
	if messages, ok := req["messages"].([]interface{}); ok {
		for _, msg := range messages {
			msgMap := msg.(map[string]interface{})
			unified.Messages = append(unified.Messages, UnifiedMessage{
				Role:      getString(msgMap, "role"),
				Content:   msgMap["content"],
				ToolCalls: parseOpenAIToolCalls(msgMap),
			})
		}
	}

	// 转换工具
	if tools, ok := req["tools"].([]interface{}); ok {
		for _, tool := range tools {
			toolMap := tool.(map[string]interface{})
			if funcMap, ok := toolMap["function"].(map[string]interface{}); ok {
				unified.Tools = append(unified.Tools, UnifiedTool{
					Type: "function",
					Function: UnifiedFunc{
						Name:        getString(funcMap, "name"),
						Description: getString(funcMap, "description"),
						Parameters:  funcMap["parameters"],
					},
				})
			}
		}
	}

	return unified, nil
}

// TransformUnifiedToOpenAI 将统一格式转换为 OpenAI 格式
func TransformUnifiedToOpenAI(unified *UnifiedRequest) ([]byte, error) {
	req := map[string]interface{}{
		"model":    unified.Model,
		"messages": []interface{}{},
		"stream":   unified.Stream,
	}

	if unified.MaxTokens > 0 {
		req["max_tokens"] = unified.MaxTokens
	}
	if unified.Temperature != nil {
		req["temperature"] = *unified.Temperature
	}
	if unified.TopP != nil {
		req["top_p"] = *unified.TopP
	}

	// 转换消息
	messages := []interface{}{}
	for _, msg := range unified.Messages {
		msgMap := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Content != nil {
			msgMap["content"] = msg.Content
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := []interface{}{}
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			msgMap["tool_calls"] = toolCalls
		}
		messages = append(messages, msgMap)
	}
	req["messages"] = messages

	// 转换工具
	if len(unified.Tools) > 0 {
		tools := []interface{}{}
		for _, tool := range unified.Tools {
			tools = append(tools, map[string]interface{}{
				"type": tool.Type,
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			})
		}
		req["tools"] = tools
	}

	if unified.Stream {
		req["stream_options"] = map[string]interface{}{"include_usage": true}
	}

	return json.Marshal(req)
}

// TransformProviderResponse 转换供应商响应
func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
	if providerType == clientType {
		return response, nil
	}

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()

	// 检查是否是流式响应
	contentType := response.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if isStream {
		return transformStreamResponse(response, body, providerType, clientType)
	}

	return transformNonStreamResponse(response, body, providerType, clientType)
}

func transformNonStreamResponse(response *http.Response, body []byte, providerType, clientType string) (*http.Response, error) {
	var unified *UnifiedResponse
	var err error

	// 供应商格式 -> 统一格式
	switch providerType {
	case "openai":
		unified, err = parseOpenAIResponse(body)
	case "anthropic":
		unified, err = parseAnthropicResponse(body)
	default:
		unified, err = parseOpenAIResponse(body)
	}

	if err != nil {
		return nil, err
	}

	// 统一格式 -> 客户端格式
	var newBody []byte
	switch clientType {
	case "openai":
		newBody, err = formatOpenAIResponse(unified)
	case "anthropic":
		newBody, err = formatAnthropicResponse(unified)
	default:
		newBody, err = formatOpenAIResponse(unified)
	}

	if err != nil {
		return nil, err
	}

	// 创建新响应
	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        response.Header.Clone(),
		Body:          io.NopCloser(strings.NewReader(string(newBody))),
		ContentLength: int64(len(newBody)),
	}

	return newResponse, nil
}

func transformStreamResponse(response *http.Response, body []byte, providerType, clientType string) (*http.Response, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		// 如果格式相同，直接透传
		if providerType == clientType {
			pw.Write(body)
			return
		}

		scanner := strings.Split(string(body), "\n")
		var textBuffer strings.Builder

		for _, line := range scanner {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// 处理 SSE 格式
			if !strings.HasPrefix(line, "data: ") {
				if strings.HasPrefix(line, "event:") {
					continue
				}
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "" || data == "[DONE]" {
				// 发送最终的 [DONE]
				fmt.Fprintf(pw, "data: [DONE]\n\n")
				continue
			}

			// 解析 JSON 数据
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// Anthropic → OpenAI 转换
			if providerType == "anthropic" && clientType == "openai" {
				eventType := getString(chunk, "type")

				switch eventType {
				case "message_start":
					// 忽略 message_start
					continue

				case "content_block_start":
					// 忽略 content_block_start
					continue

				case "ping":
					// 忽略 ping
					continue

				case "content_block_delta":
					// 提取文本内容
					if delta, ok := chunk["delta"].(map[string]interface{}); ok {
						if text := getString(delta, "text"); text != "" {
							textBuffer.WriteString(text)

							// 构造 OpenAI 流式响应
							openaiChunk := map[string]interface{}{
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

							chunkData, _ := json.Marshal(openaiChunk)
							fmt.Fprintf(pw, "data: %s\n\n", string(chunkData))
						}
					}

				case "content_block_stop":
					// 忽略 content_block_stop
					continue

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

					// 构造最终的 OpenAI 响应
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

					chunkData, _ := json.Marshal(finalChunk)
					fmt.Fprintf(pw, "data: %s\n\n", string(chunkData))

				case "message_stop":
					// 发送 [DONE]
					fmt.Fprintf(pw, "data: [DONE]\n\n")
				}
			} else if providerType == "openai" && clientType == "anthropic" {
				// OpenAI → Anthropic 转换
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					choice := choices[0].(map[string]interface{})

					// 检查是否有 delta
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						// 处理角色信息（第一个chunk）
						if role := getString(delta, "role"); role != "" {
							// 发送 message_start 事件
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
							startData, _ := json.Marshal(messageStart)
							fmt.Fprintf(pw, "event: message_start\ndata: %s\n\n", string(startData))

							// 发送 content_block_start
							blockStart := map[string]interface{}{
								"type":  "content_block_start",
								"index": 0,
								"content_block": map[string]interface{}{
									"type": "text",
									"text": "",
								},
							}
							blockData, _ := json.Marshal(blockStart)
							fmt.Fprintf(pw, "event: content_block_start\ndata: %s\n\n", string(blockData))
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
							deltaData, _ := json.Marshal(contentDelta)
							fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(deltaData))
						}

						// 处理结束
						if finishReason := getString(choice, "finish_reason"); finishReason != "" {
							// 发送 content_block_stop
							blockStop := map[string]interface{}{
								"type":  "content_block_stop",
								"index": 0,
							}
							stopData, _ := json.Marshal(blockStop)
							fmt.Fprintf(pw, "event: content_block_stop\ndata: %s\n\n", string(stopData))

							// 转换 finish_reason
							stopReason := "end_turn"
							if finishReason == "tool_calls" {
								stopReason = "tool_use"
							} else if finishReason == "length" {
								stopReason = "max_tokens"
							}

							// 发送 message_delta
							messageDelta := map[string]interface{}{
								"type": "message_delta",
								"delta": map[string]interface{}{
									"stop_reason": stopReason,
								},
							}

							// 添加 usage 信息（Anthropic 格式）
							if usage, ok := chunk["usage"].(map[string]interface{}); ok {
								inputTokens := int(getFloat(usage, "prompt_tokens"))
								outputTokens := int(getFloat(usage, "completion_tokens"))
								messageDelta["usage"] = map[string]interface{}{
									"input_tokens":  inputTokens,
									"output_tokens": outputTokens,
								}
							}

							deltaData, _ := json.Marshal(messageDelta)
							fmt.Fprintf(pw, "event: message_delta\ndata: %s\n\n", string(deltaData))

							// 发送 message_stop
							messageStop := map[string]interface{}{
								"type": "message_stop",
							}
							stopMsgData, _ := json.Marshal(messageStop)
							fmt.Fprintf(pw, "event: message_stop\ndata: %s\n\n", string(stopMsgData))
						}
					}
				}
			} else {
				// 其他转换场景，暂时直接透传
				newData, _ := json.Marshal(chunk)
				fmt.Fprintf(pw, "data: %s\n\n", string(newData))
			}
		}
	}()

	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        response.Header.Clone(),
		Body:          pr,
		ContentLength: -1,
	}

	return newResponse, nil
}

func parseOpenAIResponse(body []byte) (*UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &UnifiedResponse{
		ID:      getString(resp, "id"),
		Object:  getString(resp, "object"),
		Created: int64(getFloat(resp, "created")),
		Model:   getString(resp, "model"),
	}

	if choices, ok := resp["choices"].([]interface{}); ok && len(choices) > 0 {
		choice := choices[0].(map[string]interface{})
		msg := choice["message"].(map[string]interface{})

		unified.Choices = []UnifiedChoice{{
			Index: 0,
			Message: &UnifiedMessage{
				Role:      getString(msg, "role"),
				Content:   msg["content"],
				ToolCalls: parseOpenAIToolCalls(msg),
			},
			FinishReason: getString(choice, "finish_reason"),
		}}
	}

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		unified.Usage = &models.Usage{
			PromptTokens:     int64(getFloat(usage, "prompt_tokens")),
			CompletionTokens: int64(getFloat(usage, "completion_tokens")),
			TotalTokens:      int64(getFloat(usage, "total_tokens")),
		}
	}

	return unified, nil
}

func formatOpenAIResponse(unified *UnifiedResponse) ([]byte, error) {
	resp := map[string]interface{}{
		"id":      unified.ID,
		"object":  unified.Object,
		"created": unified.Created,
		"model":   unified.Model,
		"choices": []interface{}{},
	}

	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		msg := map[string]interface{}{
			"role": choice.Message.Role,
		}
		if choice.Message.Content != nil {
			msg["content"] = choice.Message.Content
		}
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls := []interface{}{}
			for _, tc := range choice.Message.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			msg["tool_calls"] = toolCalls
		}

		resp["choices"] = []interface{}{
			map[string]interface{}{
				"index":         choice.Index,
				"message":       msg,
				"finish_reason": choice.FinishReason,
			},
		}
	}

	if unified.Usage != nil {
		resp["usage"] = map[string]interface{}{
			"prompt_tokens":     unified.Usage.PromptTokens,
			"completion_tokens": unified.Usage.CompletionTokens,
			"total_tokens":      unified.Usage.TotalTokens,
		}
	}

	return json.Marshal(resp)
}

func parseOpenAIToolCalls(msgMap map[string]interface{}) []UnifiedToolCall {
	var toolCalls []UnifiedToolCall
	if tcs, ok := msgMap["tool_calls"].([]interface{}); ok {
		for _, tc := range tcs {
			tcMap := tc.(map[string]interface{})
			funcMap := tcMap["function"].(map[string]interface{})
			toolCalls = append(toolCalls, UnifiedToolCall{
				ID:   getString(tcMap, "id"),
				Type: getString(tcMap, "type"),
				Function: UnifiedToolCallFunction{
					Name:      getString(funcMap, "name"),
					Arguments: getString(funcMap, "arguments"),
				},
			})
		}
	}
	return toolCalls
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}
