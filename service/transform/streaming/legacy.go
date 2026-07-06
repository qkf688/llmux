package streaming

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atopos31/llmio/common/maputil"
)

// TransformStreamResponse is the legacy whole-body stream transformer.
// It is preserved for compatibility but no longer used by the real-time router.
func TransformStreamResponse(response *http.Response, body []byte, providerType, clientType string) (*http.Response, error) {
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
				eventType := maputil.String(chunk, "type")

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
						if text := maputil.String(delta, "text"); text != "" {
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
						if reason := maputil.String(delta, "stop_reason"); reason != "" {
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
							"prompt_tokens":     int(maputil.Float64(usage, "input_tokens")),
							"completion_tokens": int(maputil.Float64(usage, "output_tokens")),
							"total_tokens":      int(maputil.Float64(usage, "input_tokens") + maputil.Float64(usage, "output_tokens")),
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
						if role := maputil.String(delta, "role"); role != "" {
							// 发送 message_start 事件
							messageStart := map[string]interface{}{
								"type": "message_start",
								"message": map[string]interface{}{
									"id":      maputil.String(chunk, "id"),
									"type":    "message",
									"role":    "assistant",
									"content": []interface{}{},
									"model":   maputil.String(chunk, "model"),
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
						if content := maputil.String(delta, "content"); content != "" {
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
						if finishReason := maputil.String(choice, "finish_reason"); finishReason != "" {
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
								inputTokens := int(maputil.Float64(usage, "prompt_tokens"))
								outputTokens := int(maputil.Float64(usage, "completion_tokens"))
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
