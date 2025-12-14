package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
		// 先统计非 system 消息的数量
		nonSystemCount := 0
		for _, msg := range messages {
			msgMap := msg.(map[string]interface{})
			if getString(msgMap, "role") != "system" {
				nonSystemCount++
			}
		}

		// 只有在有非 system 消息时才提取 system 消息
		// 否则保持原样以便提供商返回合适的错误
		extractSystem := nonSystemCount > 0

		for _, msg := range messages {
			msgMap := msg.(map[string]interface{})
			role := getString(msgMap, "role")

			// 只在有其他消息时才提取 system 消息
			if role == "system" && extractSystem {
				if content, ok := msgMap["content"].(string); ok && content != "" {
					if unified.System != "" {
						unified.System += "\n\n" + content
					} else {
						unified.System = content
					}
				}
				continue // 不将 system 消息添加到 messages 数组
			}

			msg := UnifiedMessage{
				Role:      role,
				Content:   msgMap["content"],
				ToolCalls: parseOpenAIToolCalls(msgMap),
			}

			// 处理 tool 角色消息的 tool_call_id
			if role == "tool" {
				if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
					msg.ToolCallID = toolCallID
				}
			}

			unified.Messages = append(unified.Messages, msg)
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

	// 如果有 system 字段,添加为第一条消息
	if unified.System != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": unified.System,
		})
	}

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
		// 处理 tool 角色消息的 tool_call_id
		if msg.Role == "tool" && msg.ToolCallID != "" {
			msgMap["tool_call_id"] = msg.ToolCallID
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

	// 检查是否是流式响应
	contentType := response.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if isStream {
		// 流式响应：直接从 Body 读取器进行实时转换
		return transformStreamResponseRealtime(response, providerType, clientType)
	}

	// 非流式响应：读取完整响应体后转换
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()

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

// transformStreamResponseRealtime 实时流式响应转换（直接从 Body 读取器转换）
func transformStreamResponseRealtime(response *http.Response, providerType, clientType string) (*http.Response, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer response.Body.Close()

		scanner := bufio.NewScanner(response.Body)
		// 增加初始缓冲区大小到 64KB，最大 15MB（与 process.go 一致）
		scanner.Buffer(make([]byte, 0, 64*1024), 15*1024*1024)

		var currentEvent string
		lineCount := 0
		errorCount := 0

		for scanner.Scan() {
			lineCount++
			line := scanner.Text()
			if line == "" {
				// 空行是 SSE 消息分隔符
				currentEvent = "" // 重置事件类型
				continue
			}

			// 处理 event 行（记录事件类型）- 兼容带空格和不带空格两种格式
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			// 处理 data 行 - 兼容带空格和不带空格两种格式
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}

			// Anthropic → OpenAI 转换
			if providerType == "anthropic" && clientType == "openai" {
				if data == "[DONE]" {
					if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
						slog.Error("failed to write to pipe in stream transformation",
							"provider_type", providerType,
							"client_type", clientType,
							"line", lineCount,
							"error", err)
						pw.CloseWithError(err)
						return
					}
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"data_length", len(data),
						"error", err,
						"data_preview", func() string {
							if len(data) > 100 {
								return data[:100]
							}
							return data
						}())
					continue
				}

				// 优先使用 event 行的事件类型,如果没有则从 JSON 中获取
				eventType := currentEvent
				if eventType == "" {
					eventType = getString(chunk, "type")
				}

				switch eventType {
				case "message_start", "ping":
					// 忽略这些事件
					continue

				case "content_block_start":
					// 处理工具调用开始
					if contentBlock, ok := chunk["content_block"].(map[string]interface{}); ok {
						blockType := getString(contentBlock, "type")
						if blockType == "tool_use" {
							// 工具调用开始，发送角色信息
							openaiChunk := map[string]interface{}{
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
							chunkData, _ := json.Marshal(openaiChunk)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
							}
						}
					}
					continue

				case "content_block_delta":
					// 提取文本内容或工具调用参数并转换为 OpenAI 格式
					if delta, ok := chunk["delta"].(map[string]interface{}); ok {
						deltaType := getString(delta, "type")

						if deltaType == "text_delta" {
							if text := getString(delta, "text"); text != "" {
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
								if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
									slog.Error("failed to write to pipe in stream transformation",
										"provider_type", providerType,
										"client_type", clientType,
										"line", lineCount,
										"error", err)
									pw.CloseWithError(err)
									return
								}
							}
						} else if deltaType == "input_json_delta" {
							// 工具调用参数增量
							if partialJson := getString(delta, "partial_json"); partialJson != "" {
								openaiChunk := map[string]interface{}{
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
															"arguments": partialJson,
														},
													},
												},
											},
											"finish_reason": nil,
										},
									},
								}
								chunkData, _ := json.Marshal(openaiChunk)
								if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
									slog.Error("failed to write to pipe in stream transformation",
										"provider_type", providerType,
										"client_type", clientType,
										"line", lineCount,
										"error", err)
									pw.CloseWithError(err)
									return
								}
							}
						}
					}

				case "content_block_stop":
					// 忽略内容块停止事件
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
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
						slog.Error("failed to write to pipe in stream transformation",
							"provider_type", providerType,
							"client_type", clientType,
							"line", lineCount,
							"error", err)
						pw.CloseWithError(err)
						return
					}

				case "message_stop":
					// 发送 [DONE]
					if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
						slog.Error("failed to write to pipe in stream transformation",
							"provider_type", providerType,
							"client_type", clientType,
							"line", lineCount,
							"error", err)
						pw.CloseWithError(err)
						return
					}
				}
			} else if providerType == "openai" && clientType == "anthropic" {
				// OpenAI → Anthropic 转换
				if data == "[DONE]" {
					messageStop := map[string]interface{}{"type": "message_stop"}
					stopData, _ := json.Marshal(messageStop)
					if _, err := fmt.Fprintf(pw, "event: message_stop\ndata: %s\n\n", string(stopData)); err != nil {
						slog.Error("failed to write to pipe in stream transformation",
							"provider_type", providerType,
							"client_type", clientType,
							"line", lineCount,
							"error", err)
						pw.CloseWithError(err)
						return
					}
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"data_length", len(data),
						"error", err,
						"data_preview", func() string {
							if len(data) > 100 {
								return data[:100]
							}
							return data
						}())
					continue
				}

				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					choice := choices[0].(map[string]interface{})

					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						// 处理角色信息（第一个chunk）
						if role := getString(delta, "role"); role != "" {
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
							if _, err := fmt.Fprintf(pw, "event: message_start\ndata: %s\n\n", string(startData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
							}

							blockStart := map[string]interface{}{
								"type":  "content_block_start",
								"index": 0,
								"content_block": map[string]interface{}{
									"type": "text",
									"text": "",
								},
							}
							blockData, _ := json.Marshal(blockStart)
							if _, err := fmt.Fprintf(pw, "event: content_block_start\ndata: %s\n\n", string(blockData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
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
							contentDeltaData, _ := json.Marshal(contentDelta)
							if _, err := fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(contentDeltaData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
							}
						}

						// 处理结束
						if finishReason := getString(choice, "finish_reason"); finishReason != "" {
							blockStop := map[string]interface{}{
								"type":  "content_block_stop",
								"index": 0,
							}
							stopData, _ := json.Marshal(blockStop)
							if _, err := fmt.Fprintf(pw, "event: content_block_stop\ndata: %s\n\n", string(stopData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
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

							messageDeltaData, _ := json.Marshal(messageDelta)
							if _, err := fmt.Fprintf(pw, "event: message_delta\ndata: %s\n\n", string(messageDeltaData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
							}

							messageStop := map[string]interface{}{"type": "message_stop"}
							stopMsgData, _ := json.Marshal(messageStop)
							if _, err := fmt.Fprintf(pw, "event: message_stop\ndata: %s\n\n", string(stopMsgData)); err != nil {
								slog.Error("failed to write to pipe in stream transformation",
									"provider_type", providerType,
									"client_type", clientType,
									"line", lineCount,
									"error", err)
								pw.CloseWithError(err)
								return
							}
						}
					}
				}
			} else {
				// 其他场景：直接透传
				if _, err := fmt.Fprintf(pw, "data: %s\n\n", data); err != nil {
					slog.Error("failed to write to pipe in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"error", err)
					pw.CloseWithError(err)
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Error("scanner error in stream transformation",
				"provider_type", providerType,
				"client_type", clientType,
				"lines_processed", lineCount,
				"errors_encountered", errorCount,
				"error", err)
			pw.CloseWithError(err)
		} else {
			slog.Debug("stream transformation completed",
				"provider_type", providerType,
				"client_type", clientType,
				"lines_processed", lineCount,
				"errors_encountered", errorCount)
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

// transformStreamResponse 流式响应转换（从完整 body 转换，用于兼容旧代码）
func transformStreamResponse(response *http.Response, body []byte, providerType, clientType string) (*http.Response, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		// 如果格式相同，逐行透传以保持流式特性
		if providerType == clientType {
			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(pw, "%s\n", line)
				}
			}
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
			tcMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			funcMap, ok := tcMap["function"].(map[string]interface{})
			if !ok {
				continue
			}

			argsStr := getString(funcMap, "arguments")
			if argsStr == "" {
				argsStr = "{}"
			}

			toolCalls = append(toolCalls, UnifiedToolCall{
				ID:   getString(tcMap, "id"),
				Type: getString(tcMap, "type"),
				Function: UnifiedToolCallFunction{
					Name:      getString(funcMap, "name"),
					Arguments: argsStr,
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
