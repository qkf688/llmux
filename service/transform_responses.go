package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atopos31/llmio/models"
)

// TransformResponsesToUnified 将 Responses API 格式转换为统一格式
func TransformResponsesToUnified(rawBody []byte) (*UnifiedRequest, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, fmt.Errorf("failed to parse responses request: %w", err)
	}

	unified := &UnifiedRequest{
		Model:  req.Model,
		System: req.Instructions,
	}

	// 处理 stream
	if req.Stream != nil {
		unified.Stream = *req.Stream
	}

	// 处理 max_output_tokens → max_tokens
	if req.MaxOutputTokens != nil {
		unified.MaxTokens = *req.MaxOutputTokens
	}

	// 处理 temperature
	if req.Temperature != nil {
		unified.Temperature = req.Temperature
	}

	// 处理 top_p
	if req.TopP != nil {
		unified.TopP = req.TopP
	}

	// 转换 input → messages
	unified.Messages = convertResponsesInputToMessages(req.Input)

	// 转换 tools
	if len(req.Tools) > 0 {
		unified.Tools = convertResponsesToolsToUnified(req.Tools)
	}

	return unified, nil
}

// convertResponsesInputToMessages 将 Responses input 转换为 UnifiedMessage 数组
func convertResponsesInputToMessages(input ResponsesInput) []UnifiedMessage {
	var messages []UnifiedMessage

	// 简单情况：input 是 string
	if input.Text != nil {
		messages = append(messages, UnifiedMessage{
			Role:    "user",
			Content: *input.Text,
		})
		return messages
	}

	// 复杂情况：input 是 array of ResponsesItem
	for _, item := range input.Items {
		switch item.Type {
		case "message", "input_text", "":
			// Cherry Studio / octopus compatibility: role-based format
			if item.Role != "" {
				msg := UnifiedMessage{Role: item.Role}

				// 处理 content
				if item.Content != nil {
					// content 可能是 string 或 array
					switch v := item.Content.(type) {
					case string:
						msg.Content = v
					case []interface{}:
						// content 是数组，提取 text
						var textParts []string
						for _, part := range v {
							if partMap, ok := part.(map[string]interface{}); ok {
								if partType, _ := partMap["type"].(string); partType == "input_text" || partType == "text" {
									if text, ok := partMap["text"].(string); ok {
										textParts = append(textParts, text)
									}
								}
							}
						}
						if len(textParts) > 0 {
							msg.Content = strings.Join(textParts, "")
						}
					}
				} else if item.Text != nil {
					msg.Content = *item.Text
				}

				messages = append(messages, msg)
			} else if item.Text != nil {
				// 兼容原有 type-based 格式
				messages = append(messages, UnifiedMessage{
					Role:    "user",
					Content: *item.Text,
				})
			}

		case "output_text":
			// output_text → assistant message (历史对话中的助手回复)
			if item.Text != nil {
				messages = append(messages, UnifiedMessage{
					Role:    "assistant",
					Content: *item.Text,
				})
			}

		case "function_call":
			// function_call → assistant message with tool_calls
			if item.Name != nil && item.Arguments != nil {
				toolCall := UnifiedToolCall{
					Type: "function",
					Function: UnifiedToolCallFunction{
						Name:      *item.Name,
						Arguments: *item.Arguments,
					},
				}
				if item.CallID != nil {
					toolCall.ID = *item.CallID
				} else if item.ID != "" {
					toolCall.ID = item.ID
				}

				messages = append(messages, UnifiedMessage{
					Role:      "assistant",
					ToolCalls: []UnifiedToolCall{toolCall},
				})
			}

		case "function_call_output":
			// function_call_output → tool message
			if item.Output != nil {
				msg := UnifiedMessage{
					Role:    "tool",
					Content: *item.Output,
				}
				if item.CallID != nil {
					msg.ToolCallID = *item.CallID
				}
				messages = append(messages, msg)
			}
		}
	}

	return messages
}

// convertResponsesToolsToUnified 将 Responses tools 转换为 UnifiedTool 数组
func convertResponsesToolsToUnified(tools []ResponsesTool) []UnifiedTool {
	var unified []UnifiedTool

	for _, tool := range tools {
		if tool.Type == "function" && tool.Function != nil {
			unified = append(unified, UnifiedTool{
				Type: "function",
				Function: UnifiedFunc{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			})
		}
	}

	return unified
}

// TransformUnifiedToResponses 将统一格式转换为 Responses API 格式
func TransformUnifiedToResponses(unified *UnifiedRequest) ([]byte, error) {
	req := ResponsesRequest{
		Model:        unified.Model,
		Instructions: unified.System,
	}

	// 处理 stream
	if unified.Stream {
		stream := true
		req.Stream = &stream
	}

	// 处理 max_tokens → max_output_tokens
	if unified.MaxTokens > 0 {
		req.MaxOutputTokens = &unified.MaxTokens
	}

	// 处理 temperature
	if unified.Temperature != nil {
		req.Temperature = unified.Temperature
	}

	// 处理 top_p
	if unified.TopP != nil {
		req.TopP = unified.TopP
	}

	// 转换 messages → input
	req.Input = convertMessagesToResponsesInput(unified.Messages)

	// 转换 tools
	if len(unified.Tools) > 0 {
		req.Tools = convertUnifiedToolsToResponses(unified.Tools)
	}

	return json.Marshal(req)
}

// convertMessagesToResponsesInput 将 UnifiedMessage 数组转换为 ResponsesInput
func convertMessagesToResponsesInput(messages []UnifiedMessage) ResponsesInput {
	var items []ResponsesItem

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			// user message → input_text
			if content, ok := msg.Content.(string); ok {
				items = append(items, ResponsesItem{
					Type: "input_text",
					Text: &content,
				})
			}

		case "assistant":
			// assistant message with tool_calls → function_call
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					items = append(items, ResponsesItem{
						Type:      "function_call",
						ID:        tc.ID,
						CallID:    &tc.ID,
						Name:      &tc.Function.Name,
						Arguments: &tc.Function.Arguments,
					})
				}
			} else if content, ok := msg.Content.(string); ok {
				// assistant message without tool_calls → output_text
				items = append(items, ResponsesItem{
					Type: "output_text",
					Text: &content,
				})
			}

		case "tool":
			// tool message → function_call_output
			if content, ok := msg.Content.(string); ok {
				items = append(items, ResponsesItem{
					Type:   "function_call_output",
					CallID: &msg.ToolCallID,
					Output: &content,
				})
			}
		}
	}

	return ResponsesInput{Items: items}
}

// convertUnifiedToolsToResponses 将 UnifiedTool 数组转换为 ResponsesTool 数组
func convertUnifiedToolsToResponses(tools []UnifiedTool) []ResponsesTool {
	var responses []ResponsesTool

	for _, tool := range tools {
		if tool.Type == "function" {
			params, _ := tool.Function.Parameters.(map[string]interface{})
			responses = append(responses, ResponsesTool{
				Type: "function",
				Function: &ResponsesToolFunction{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  params,
				},
			})
		}
	}

	return responses
}

// parseResponsesResponse 将 Responses API 响应转换为统一格式
func parseResponsesResponse(body []byte) (*UnifiedResponse, error) {
	var resp ResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse responses response: %w", err)
	}

	unified := &UnifiedResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.CreatedAt,
		Model:   resp.Model,
	}

	// 转换 output → choices
	if len(resp.Output) > 0 {
		choice := UnifiedChoice{
			Index: 0,
		}

		// 构建 message
		msg := &UnifiedMessage{
			Role: "assistant",
		}

		var textContent string
		var toolCalls []UnifiedToolCall

		for _, item := range resp.Output {
			switch item.Type {
			case "output_text":
				if item.Text != nil {
					textContent += *item.Text
				}

			case "function_call":
				if item.Name != nil && item.Arguments != nil {
					tc := UnifiedToolCall{
						Type: "function",
						Function: UnifiedToolCallFunction{
							Name:      *item.Name,
							Arguments: *item.Arguments,
						},
					}
					if item.CallID != nil {
						tc.ID = *item.CallID
					} else if item.ID != "" {
						tc.ID = item.ID
					}
					toolCalls = append(toolCalls, tc)
				}
			}
		}

		if textContent != "" {
			msg.Content = textContent
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}

		choice.Message = msg

		// 转换 status → finish_reason
		if resp.Status != nil {
			switch *resp.Status {
			case "completed":
				if len(toolCalls) > 0 {
					choice.FinishReason = "tool_calls"
				} else {
					choice.FinishReason = "stop"
				}
			case "incomplete":
				choice.FinishReason = "length"
			case "failed":
				choice.FinishReason = "error"
			}
		}

		unified.Choices = []UnifiedChoice{choice}
	}

	// 转换 usage
	if resp.Usage != nil {
		unified.Usage = &models.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.InputTokenDetails != nil {
			unified.Usage.PromptTokensDetails.CachedTokens = resp.Usage.InputTokenDetails.CachedTokens
		}
	}

	return unified, nil
}

// formatResponsesResponse 将统一格式转换为 Responses API 响应
func formatResponsesResponse(unified *UnifiedResponse) ([]byte, error) {
	resp := ResponsesResponse{
		Object:    "response",
		ID:        unified.ID,
		Model:     unified.Model,
		CreatedAt: unified.Created,
	}

	// 转换 choices → output
	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		var output []ResponsesItem

		if choice.Message != nil {
			// 处理工具调用
			for _, tc := range choice.Message.ToolCalls {
				output = append(output, ResponsesItem{
					Type:      "function_call",
					ID:        tc.ID,
					CallID:    &tc.ID,
					Name:      &tc.Function.Name,
					Arguments: &tc.Function.Arguments,
				})
			}

			// 处理文本内容 - 使用 message 包裹 output_text
			if content, ok := choice.Message.Content.(string); ok && content != "" {
				text := content
				contentItems := []ResponsesItem{{
					Type:        "output_text",
					Text:        &text,
					Annotations: []ResponsesAnnotation{},
				}}
				status := "completed"
				msgID := fmt.Sprintf("msg_%s", unified.ID)
				output = append(output, ResponsesItem{
					ID:      msgID,
					Type:    "message",
					Role:    "assistant",
					Status:  &status,
					Content: &ResponsesInput{Items: contentItems},
				})
			}
		}

		resp.Output = output

		// 转换 finish_reason → status
		status := "completed"
		switch choice.FinishReason {
		case "stop", "tool_calls":
			status = "completed"
		case "length":
			status = "incomplete"
		case "error":
			status = "failed"
		}
		resp.Status = &status
	}

	// 转换 usage
	if unified.Usage != nil {
		resp.Usage = &ResponsesUsage{
			InputTokens:  unified.Usage.PromptTokens,
			OutputTokens: unified.Usage.CompletionTokens,
			TotalTokens:  unified.Usage.TotalTokens,
		}
		if unified.Usage.PromptTokensDetails.CachedTokens > 0 {
			resp.Usage.InputTokenDetails = &ResponsesInputTokenDetails{
				CachedTokens: unified.Usage.PromptTokensDetails.CachedTokens,
			}
		}
	}

	return json.Marshal(resp)
}
