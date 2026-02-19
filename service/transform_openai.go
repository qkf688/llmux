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

	// 验证 messages 不为空
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty: OpenAI API requires at least one message")
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
	case "openai-res":
		unified, err = parseResponsesResponse(body)
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
	case "openai-res":
		newBody, err = formatResponsesResponse(unified)
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
		sequenceNumber := 0
		accumulatedText := ""
		var responseID string
		var itemID string

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
			} else if providerType == "anthropic" && clientType == "openai-res" {
				// Anthropic → Responses 转换
				if data == "[DONE]" {
					// Anthropic 不发送 [DONE]，忽略
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"error", err)
					continue
				}

				eventType := currentEvent
				if eventType == "" {
					eventType = getString(chunk, "type")
				}

				switch eventType {
				case "message_start":
					// 获取 message ID 并生成 item ID
					responseID = getNestedString(chunk, "message.id")
					itemID = fmt.Sprintf("msg_%s", responseID)

					// 发送 response.created 事件
					responseCreated := map[string]interface{}{
						"type":            "response.created",
						"sequence_number": sequenceNumber,
						"response": map[string]interface{}{
							"object":     "response",
							"id":         responseID,
							"model":      getNestedString(chunk, "message.model"),
							"created_at": 0,
							"output":     []interface{}{},
							"status":     "in_progress",
						},
					}
					sequenceNumber++
					createdData, _ := marshalWithTypeFirst(responseCreated)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(createdData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					// 发送 response.in_progress 事件
					responseInProgress := map[string]interface{}{
						"type":            "response.in_progress",
						"sequence_number": sequenceNumber,
						"response": map[string]interface{}{
							"object":     "response",
							"id":         responseID,
							"model":      getNestedString(chunk, "message.model"),
							"created_at": 0,
							"output":     []interface{}{},
							"status":     "in_progress",
						},
					}
					sequenceNumber++
					inProgressData, _ := marshalWithTypeFirst(responseInProgress)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(inProgressData)); err != nil {
						pw.CloseWithError(err)
						return
					}

				case "content_block_start":
					// 发送 response.output_item.added 事件
					if contentBlock, ok := chunk["content_block"].(map[string]interface{}); ok {
						_ = getString(contentBlock, "type") // 忽略 blockType，统一使用 message 类型
						itemAdded := map[string]interface{}{
							"type":            "response.output_item.added",
							"sequence_number": sequenceNumber,
							"output_index":    int(getFloat(chunk, "index")),
							"item": map[string]interface{}{
								"id":      itemID,
								"type":    "message",
								"role":    "assistant",
								"content": []interface{}{},
								"status":  "in_progress",
							},
						}
						sequenceNumber++
						addedData, _ := marshalWithTypeFirst(itemAdded)
						if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(addedData)); err != nil {
							pw.CloseWithError(err)
							return
						}

						// 发送 response.content_part.added 事件
						contentPartAdded := map[string]interface{}{
							"type":            "response.content_part.added",
							"sequence_number": sequenceNumber,
							"output_index":    int(getFloat(chunk, "index")),
							"item_id":         itemID,
							"content_index":   0,
							"part": map[string]interface{}{
								"type": "output_text",
								"text": "",
							},
						}
						sequenceNumber++
						partAddedData, _ := marshalWithTypeFirst(contentPartAdded)
						if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(partAddedData)); err != nil {
							pw.CloseWithError(err)
							return
						}

						// 重置累积文本
						accumulatedText = ""
					}

				case "content_block_delta":
					if delta, ok := chunk["delta"].(map[string]interface{}); ok {
						deltaType := getString(delta, "type")

						if deltaType == "text_delta" {
							// 发送 response.output_text.delta 事件
							if text := getString(delta, "text"); text != "" {
								accumulatedText += text
								textDelta := map[string]interface{}{
									"type":            "response.output_text.delta",
									"sequence_number": sequenceNumber,
									"output_index":    int(getFloat(chunk, "index")),
									"item_id":         itemID,
									"content_index":   0,
									"delta":           text,
								}
								sequenceNumber++
								deltaData, _ := marshalWithTypeFirst(textDelta)
								if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(deltaData)); err != nil {
									pw.CloseWithError(err)
									return
								}
							}
						} else if deltaType == "input_json_delta" {
							// 发送 response.function_call_arguments.delta 事件
							if partialJson := getString(delta, "partial_json"); partialJson != "" {
								argsDelta := map[string]interface{}{
									"type":            "response.function_call_arguments.delta",
									"sequence_number": sequenceNumber,
									"output_index":    int(getFloat(chunk, "index")),
									"delta":           partialJson,
								}
								sequenceNumber++
								deltaData, _ := marshalWithTypeFirst(argsDelta)
								if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(deltaData)); err != nil {
									pw.CloseWithError(err)
									return
								}
							}
						}
					}

				case "message_delta":
					// 发送 response.output_text.done 事件
					outputTextDone := map[string]interface{}{
						"type":            "response.output_text.done",
						"sequence_number": sequenceNumber,
						"output_index":    0,
						"item_id":         itemID,
						"content_index":   0,
						"text":            accumulatedText,
					}
					sequenceNumber++
					textDoneData, _ := marshalWithTypeFirst(outputTextDone)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(textDoneData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					// 发送 response.content_part.done 事件
					contentPartDone := map[string]interface{}{
						"type":            "response.content_part.done",
						"sequence_number": sequenceNumber,
						"output_index":    0,
						"item_id":         itemID,
						"content_index":   0,
						"part": map[string]interface{}{
							"type": "output_text",
							"text": accumulatedText,
						},
					}
					sequenceNumber++
					partDoneData, _ := marshalWithTypeFirst(contentPartDone)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(partDoneData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					// 发送 response.output_item.done 事件
					outputItemDone := map[string]interface{}{
						"type":            "response.output_item.done",
						"sequence_number": sequenceNumber,
						"output_index":    0,
						"item": map[string]interface{}{
							"id":   itemID,
							"type": "message",
							"role": "assistant",
							"content": []map[string]interface{}{
								{
									"type": "output_text",
									"text": accumulatedText,
								},
							},
							"status": "completed",
						},
					}
					sequenceNumber++
					itemDoneData, _ := marshalWithTypeFirst(outputItemDone)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(itemDoneData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					// 发送 response.completed 事件（含 usage）
					status := "completed"
					if delta, ok := chunk["delta"].(map[string]interface{}); ok {
						if reason := getString(delta, "stop_reason"); reason != "" {
							if reason == "max_tokens" {
								status = "incomplete"
							}
						}
					}

					responseCompleted := map[string]interface{}{
						"type":            "response.completed",
						"sequence_number": sequenceNumber,
						"response": map[string]interface{}{
							"object":     "response",
							"id":         responseID,
							"model":      getNestedString(chunk, "message.model"),
							"created_at": 0,
							"output":     []interface{}{},
							"status":     status,
						},
					}

					// 添加 usage 信息
					if usage, ok := chunk["usage"].(map[string]interface{}); ok {
						usageMap := map[string]interface{}{
							"input_tokens":  int(getFloat(usage, "input_tokens")),
							"output_tokens": int(getFloat(usage, "output_tokens")),
							"total_tokens":  int(getFloat(usage, "input_tokens") + getFloat(usage, "output_tokens")),
						}
						// 添加 input_tokens_details
						if inputTokens := int(getFloat(usage, "input_tokens")); inputTokens > 0 {
							usageMap["input_tokens_details"] = map[string]interface{}{
								"cached_tokens": 0,
							}
						}
						// 添加 output_tokens_details
						if outputTokens := int(getFloat(usage, "output_tokens")); outputTokens > 0 {
							usageMap["output_tokens_details"] = map[string]interface{}{
								"reasoning_tokens": 0,
							}
						}
						responseCompleted["response"].(map[string]interface{})["usage"] = usageMap
					}

					completedData, _ := marshalWithTypeFirst(responseCompleted)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(completedData)); err != nil {
						pw.CloseWithError(err)
						return
					}

				case "message_stop":
					// Responses API 不需要单独的 stop 事件
					continue
				}

			} else if providerType == "openai-res" && clientType == "anthropic" {
				// Responses → Anthropic 转换
				if data == "[DONE]" {
					// [DONE] 被忽略，因为 response.completed 已发送 message_stop
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"error", err)
					continue
				}

				eventType := currentEvent
				if eventType == "" {
					eventType = getString(chunk, "type")
				}

				switch eventType {
				case "response.created":
					// 发送 message_start 事件
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
					startData, _ := json.Marshal(messageStart)
					if _, err := fmt.Fprintf(pw, "event: message_start\ndata: %s\n\n", string(startData)); err != nil {
						pw.CloseWithError(err)
						return
					}

				case "response.output_item.added":
					// 发送 content_block_start 事件
					if item, ok := chunk["item"].(map[string]interface{}); ok {
						itemType := getString(item, "type")
						blockStart := map[string]interface{}{
							"type":  "content_block_start",
							"index": int(getFloat(chunk, "output_index")),
							"content_block": map[string]interface{}{},
						}

						// 映射 Responses function_call → Anthropic tool_use
						if itemType == "function_call" {
							blockStart["content_block"] = map[string]interface{}{
								"type": "tool_use",
								"id":   getString(item, "id"),
								"name": getString(item, "name"),
							}
						} else if itemType == "output_text" || itemType == "text" {
							blockStart["content_block"] = map[string]interface{}{
								"type": "text",
								"text": "",
							}
						} else {
							blockStart["content_block"] = map[string]interface{}{
								"type": itemType,
							}
						}

						blockData, _ := json.Marshal(blockStart)
						if _, err := fmt.Fprintf(pw, "event: content_block_start\ndata: %s\n\n", string(blockData)); err != nil {
							pw.CloseWithError(err)
							return
						}
					}

				case "response.output_text.delta":
					// 发送 content_block_delta 事件（文本）
					if delta := getString(chunk, "delta"); delta != "" {
						contentDelta := map[string]interface{}{
							"type":  "content_block_delta",
							"index": int(getFloat(chunk, "output_index")),
							"delta": map[string]interface{}{
								"type": "text_delta",
								"text": delta,
							},
						}
						deltaData, _ := json.Marshal(contentDelta)
						if _, err := fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(deltaData)); err != nil {
							pw.CloseWithError(err)
							return
						}
					}

				case "response.function_call_arguments.delta":
					// 发送 content_block_delta 事件（工具参数）
					if delta := getString(chunk, "delta"); delta != "" {
						contentDelta := map[string]interface{}{
							"type":  "content_block_delta",
							"index": int(getFloat(chunk, "output_index")),
							"delta": map[string]interface{}{
								"type":         "input_json_delta",
								"partial_json": delta,
							},
						}
						deltaData, _ := json.Marshal(contentDelta)
						if _, err := fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(deltaData)); err != nil {
							pw.CloseWithError(err)
							return
						}
					}

				case "response.completed":
					// 发送 content_block_stop 和 message_delta 事件
					stopReason := "end_turn"
					blockStopIndex := 0
					if response, ok := chunk["response"].(map[string]interface{}); ok {
						status := getString(response, "status")
						if status == "incomplete" {
							stopReason = "max_tokens"
						}
						// 检查是否有工具调用（通过 output 判断）
						if output, ok := response["output"].([]interface{}); ok && len(output) > 0 {
							blockStopIndex = len(output) - 1
							for _, item := range output {
								if itemMap, ok := item.(map[string]interface{}); ok {
									if getString(itemMap, "type") == "function_call" {
										stopReason = "tool_use"
										break
									}
								}
							}
						}
					}

					blockStop := map[string]interface{}{
						"type":  "content_block_stop",
						"index": blockStopIndex,
					}
					stopData, _ := json.Marshal(blockStop)
					if _, err := fmt.Fprintf(pw, "event: content_block_stop\ndata: %s\n\n", string(stopData)); err != nil {
						pw.CloseWithError(err)
						return
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

					deltaData, _ := json.Marshal(messageDelta)
					if _, err := fmt.Fprintf(pw, "event: message_delta\ndata: %s\n\n", string(deltaData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					messageStop := map[string]interface{}{"type": "message_stop"}
					stopMsgData, _ := json.Marshal(messageStop)
					if _, err := fmt.Fprintf(pw, "event: message_stop\ndata: %s\n\n", string(stopMsgData)); err != nil {
						pw.CloseWithError(err)
						return
					}
				}

			} else if providerType == "openai" && clientType == "openai-res" {
				// OpenAI → Responses 转换
				if data == "[DONE]" {
					// OpenAI 的 [DONE] 不需要转换
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"error", err)
					continue
				}

				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					choice := choices[0].(map[string]interface{})

					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						// 处理角色信息（第一个chunk）
						if role := getString(delta, "role"); role != "" {
							responseID = getString(chunk, "id")
							itemID = fmt.Sprintf("msg_%s", responseID)

							// response.created 事件
							responseCreated := map[string]interface{}{
								"type":            "response.created",
								"sequence_number": sequenceNumber,
								"response": map[string]interface{}{
									"id":         responseID,
									"object":     "response",
									"model":      getString(chunk, "model"),
									"created_at": int(getFloat(chunk, "created")),
									"status":     "in_progress",
									"output":     []interface{}{},
								},
							}
							sequenceNumber++
							createdData, _ := marshalWithTypeFirst(responseCreated)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(createdData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							// response.in_progress 事件
							responseInProgress := map[string]interface{}{
								"type":            "response.in_progress",
								"sequence_number": sequenceNumber,
								"response": map[string]interface{}{
									"id":         responseID,
									"object":     "response",
									"model":      getString(chunk, "model"),
									"created_at": int(getFloat(chunk, "created")),
									"status":     "in_progress",
									"output":     []interface{}{},
								},
							}
							sequenceNumber++
							inProgressData, _ := marshalWithTypeFirst(responseInProgress)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(inProgressData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							// response.output_item.added 事件
							itemAdded := map[string]interface{}{
								"type":            "response.output_item.added",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item": map[string]interface{}{
									"id":      itemID,
									"type":    "message",
									"role":    role,
									"content": []interface{}{},
									"status":  "in_progress",
								},
							}
							sequenceNumber++
							addedData, _ := marshalWithTypeFirst(itemAdded)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(addedData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							// response.content_part.added 事件
							contentPartAdded := map[string]interface{}{
								"type":            "response.content_part.added",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item_id":         itemID,
								"content_index":   0,
								"part": map[string]interface{}{
									"type": "output_text",
									"text": "",
								},
							}
							sequenceNumber++
							partAddedData, _ := marshalWithTypeFirst(contentPartAdded)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(partAddedData)); err != nil {
								pw.CloseWithError(err)
								return
							}
						}

						// 处理文本内容
						if content := getString(delta, "content"); content != "" {
							accumulatedText += content
							textDelta := map[string]interface{}{
								"type":            "response.output_text.delta",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item_id":         itemID,
								"content_index":   0,
								"delta":           content,
							}
							sequenceNumber++
							deltaData, _ := marshalWithTypeFirst(textDelta)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(deltaData)); err != nil {
								pw.CloseWithError(err)
								return
							}
						}

						// 处理工具调用
						if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
							for _, tc := range toolCalls {
								toolCall, ok := tc.(map[string]interface{})
								if !ok {
									continue
								}
								if function, ok := toolCall["function"].(map[string]interface{}); ok {
									// 首包含 id/name 时，发送 output_item.added 事件
									if id := getString(toolCall, "id"); id != "" {
										itemAdded := map[string]interface{}{
											"type":         "response.output_item.added",
											"output_index": int(getFloat(toolCall, "index")),
											"item": map[string]interface{}{
												"type":      "function_call",
												"id":        id,
												"call_id":   id,
												"name":      getString(function, "name"),
												"arguments": "",
											},
										}
										addedData, _ := json.Marshal(itemAdded)
										if _, err := fmt.Fprintf(pw, "event: response.output_item.added\ndata: %s\n\n", string(addedData)); err != nil {
											pw.CloseWithError(err)
											return
										}
									}

									// 参数增量
									if args := getString(function, "arguments"); args != "" {
										argsDelta := map[string]interface{}{
											"type":         "response.function_call_arguments.delta",
											"output_index": int(getFloat(toolCall, "index")),
											"delta":        args,
										}
										deltaData, _ := json.Marshal(argsDelta)
										if _, err := fmt.Fprintf(pw, "event: response.function_call_arguments.delta\ndata: %s\n\n", string(deltaData)); err != nil {
											pw.CloseWithError(err)
											return
										}
									}
								}
							}
						}

						// 处理结束
						if finishReason := getString(choice, "finish_reason"); finishReason != "" {
							// response.output_text.done 事件
							outputTextDone := map[string]interface{}{
								"type":            "response.output_text.done",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item_id":         itemID,
								"content_index":   0,
								"text":            accumulatedText,
							}
							sequenceNumber++
							textDoneData, _ := marshalWithTypeFirst(outputTextDone)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(textDoneData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							// response.content_part.done 事件
							contentPartDone := map[string]interface{}{
								"type":            "response.content_part.done",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item_id":         itemID,
								"content_index":   0,
								"part": map[string]interface{}{
									"type": "output_text",
									"text": accumulatedText,
								},
							}
							sequenceNumber++
							partDoneData, _ := marshalWithTypeFirst(contentPartDone)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(partDoneData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							// response.output_item.done 事件
							outputItemDone := map[string]interface{}{
								"type":            "response.output_item.done",
								"sequence_number": sequenceNumber,
								"output_index":    0,
								"item": map[string]interface{}{
									"id":   itemID,
									"type": "message",
									"role": "assistant",
									"content": []map[string]interface{}{
										{
											"type": "output_text",
											"text": accumulatedText,
										},
									},
									"status": "completed",
								},
							}
							sequenceNumber++
							itemDoneData, _ := marshalWithTypeFirst(outputItemDone)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(itemDoneData)); err != nil {
								pw.CloseWithError(err)
								return
							}

							status := "completed"
							if finishReason == "length" {
								status = "incomplete"
							}

							responseCompleted := map[string]interface{}{
								"type":            "response.completed",
								"sequence_number": sequenceNumber,
								"response": map[string]interface{}{
									"object":     "response",
									"id":         responseID,
									"model":      getString(chunk, "model"),
									"created_at": int(getFloat(chunk, "created")),
									"status":     status,
									"output":     []interface{}{},
								},
							}

							// 添加 usage 信息
							if usage, ok := chunk["usage"].(map[string]interface{}); ok {
								usageMap := map[string]interface{}{
									"input_tokens":  int(getFloat(usage, "prompt_tokens")),
									"output_tokens": int(getFloat(usage, "completion_tokens")),
									"total_tokens":  int(getFloat(usage, "total_tokens")),
								}
								// 添加 input_tokens_details
								if promptTokens := int(getFloat(usage, "prompt_tokens")); promptTokens > 0 {
									usageMap["input_tokens_details"] = map[string]interface{}{
										"cached_tokens": 0,
									}
								}
								// 添加 output_tokens_details
								if completionTokens := int(getFloat(usage, "completion_tokens")); completionTokens > 0 {
									usageMap["output_tokens_details"] = map[string]interface{}{
										"reasoning_tokens": 0,
									}
								}
								responseCompleted["response"].(map[string]interface{})["usage"] = usageMap
							}

							completedData, _ := marshalWithTypeFirst(responseCompleted)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(completedData)); err != nil {
								pw.CloseWithError(err)
								return
							}
						}
					}
				}

			} else if providerType == "openai-res" && clientType == "openai" {
				// Responses → OpenAI 转换
				if data == "[DONE]" {
					// [DONE] 被忽略，因为 response.completed 已发送 [DONE]
					continue
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					errorCount++
					slog.Error("failed to parse SSE chunk in stream transformation",
						"provider_type", providerType,
						"client_type", clientType,
						"line", lineCount,
						"error", err)
					continue
				}

				eventType := currentEvent
				if eventType == "" {
					eventType = getString(chunk, "type")
				}

				switch eventType {
				case "response.created":
					// 发送角色信息
					openaiChunk := map[string]interface{}{
						"id":      getNestedString(chunk, "response.id"),
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   "responses-api",
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
					chunkData, _ := json.Marshal(openaiChunk)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
						pw.CloseWithError(err)
						return
					}

				case "response.output_item.added":
					// 处理工具调用开始（发送 id/type/name）
					if item, ok := chunk["item"].(map[string]interface{}); ok {
						itemType := getString(item, "type")
						if itemType == "function_call" {
							openaiChunk := map[string]interface{}{
								"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
								"object":  "chat.completion.chunk",
								"created": time.Now().Unix(),
								"model":   "responses-api",
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
							chunkData, _ := json.Marshal(openaiChunk)
							if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
								pw.CloseWithError(err)
								return
							}
						}
					}

				case "response.output_text.delta":
					// 发送文本增量
					if delta := getString(chunk, "delta"); delta != "" {
						openaiChunk := map[string]interface{}{
							"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
							"object":  "chat.completion.chunk",
							"created": time.Now().Unix(),
							"model":   "responses-api",
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
						chunkData, _ := json.Marshal(openaiChunk)
						if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
							pw.CloseWithError(err)
							return
						}
					}

				case "response.function_call_arguments.delta":
					// 发送工具调用参数增量
					if delta := getString(chunk, "delta"); delta != "" {
						openaiChunk := map[string]interface{}{
							"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
							"object":  "chat.completion.chunk",
							"created": time.Now().Unix(),
							"model":   "responses-api",
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
						chunkData, _ := json.Marshal(openaiChunk)
						if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
							pw.CloseWithError(err)
							return
						}
					}

				case "response.completed":
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
						"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   "responses-api",
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

					chunkData, _ := json.Marshal(finalChunk)
					if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(chunkData)); err != nil {
						pw.CloseWithError(err)
						return
					}

					// 发送 [DONE]
					if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
						pw.CloseWithError(err)
						return
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

// getNestedString 支持点路径取值，如 "response.id"
func getNestedString(m map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，取字符串值
			if v, ok := current[part].(string); ok {
				return v
			}
			return ""
		}
		// 中间部分，继续深入
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return ""
		}
	}
	return ""
}

// getNestedFloat 支持点路径取值，如 "response.usage.input_tokens"
func getNestedFloat(m map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，取数值
			if v, ok := current[part].(float64); ok {
				return v
			}
			return 0
		}
		// 中间部分，继续深入
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return 0
		}
	}
	return 0
}

// getNestedMap 支持点路径取值，返回嵌套的 map
func getNestedMap(m map[string]interface{}, path string) map[string]interface{} {
	parts := strings.Split(path, ".")
	current := m
	for _, part := range parts {
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return nil
		}
	}
	return current
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

// fieldOrder 定义各事件类型的字段顺序
var fieldOrder = map[string][]string{
	"response.created":           {"type", "sequence_number", "response"},
	"response.in_progress":       {"type", "sequence_number", "response"},
	"response.output_item.added": {"type", "sequence_number", "output_index", "item"},
	"response.content_part.added": {"type", "sequence_number", "output_index", "item_id", "content_index", "part"},
	"response.output_text.delta":  {"type", "sequence_number", "output_index", "item_id", "content_index", "delta"},
	"response.output_text.done":   {"type", "sequence_number", "output_index", "item_id", "content_index", "text"},
	"response.content_part.done":  {"type", "sequence_number", "output_index", "item_id", "content_index", "part"},
	"response.output_item.done":   {"type", "sequence_number", "output_index", "item"},
	"response.completed":          {"type", "sequence_number", "response"},
}

// nestedFieldOrder 定义嵌套对象的字段顺序
var nestedFieldOrder = map[string][]string{
	"response":               {"object", "id", "model", "created_at", "output", "status", "usage"},
	"item":                   {"id", "type", "role", "content", "status"},
	"part":                   {"type", "text"},
	"usage":                  {"input_tokens", "input_tokens_details", "output_tokens", "output_tokens_details", "total_tokens"},
	"input_tokens_details":   {"cached_tokens"},
	"output_tokens_details":  {"reasoning_tokens"},
	"content_item":           {"type", "text"},
}

// marshalWithTypeFirst 按照预定义顺序序列化 JSON
func marshalWithTypeFirst(data map[string]interface{}) ([]byte, error) {
	eventType, hasType := data["type"].(string)
	if !hasType {
		return json.Marshal(data)
	}

	// 获取该事件类型的字段顺序
	order, exists := fieldOrder[eventType]
	if !exists {
		// 如果没有预定义顺序，type 在前，其他随机
		return marshalWithOrder(data, []string{"type"})
	}

	return marshalWithOrder(data, order)
}

// marshalWithOrder 按指定顺序序列化对象
func marshalWithOrder(data map[string]interface{}, order []string) ([]byte, error) {
	result := "{"
	first := true

	// 按顺序输出字段
	for _, key := range order {
		value, exists := data[key]
		if !exists {
			continue
		}

		if !first {
			result += ","
		}
		first = false

		// 序列化值
		valueJSON, err := marshalValue(value, key)
		if err != nil {
			return nil, err
		}

		result += fmt.Sprintf(`"%s":%s`, key, valueJSON)
	}

	// 输出未在顺序中的字段（如果有）
	for key, value := range data {
		// 检查是否已经输出过
		found := false
		for _, orderedKey := range order {
			if key == orderedKey {
				found = true
				break
			}
		}
		if found {
			continue
		}

		if !first {
			result += ","
		}
		first = false

		valueJSON, err := marshalValue(value, key)
		if err != nil {
			return nil, err
		}

		result += fmt.Sprintf(`"%s":%s`, key, valueJSON)
	}

	result += "}"
	return []byte(result), nil
}

// marshalValue 序列化值，对嵌套对象应用字段顺序
func marshalValue(value interface{}, key string) (string, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		// 检查是否有预定义的字段顺序
		if order, exists := nestedFieldOrder[key]; exists {
			data, err := marshalWithOrder(v, order)
			return string(data), err
		}
		// 没有预定义顺序，使用标准序列化
		data, err := json.Marshal(v)
		return string(data), err
	case []interface{}:
		// 处理数组
		if len(v) == 0 {
			return "[]", nil
		}

		result := "["
		for i, item := range v {
			if i > 0 {
				result += ","
			}

			// 如果是 content 数组，应用 content_item 顺序
			if key == "content" {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if order, exists := nestedFieldOrder["content_item"]; exists {
						data, err := marshalWithOrder(itemMap, order)
						if err != nil {
							return "", err
						}
						result += string(data)
						continue
					}
				}
			}

			// 其他情况使用标准序列化
			itemJSON, err := json.Marshal(item)
			if err != nil {
				return "", err
			}
			result += string(itemJSON)
		}
		result += "]"
		return result, nil
	default:
		// 基本类型使用标准序列化
		data, err := json.Marshal(v)
		return string(data), err
	}
}
