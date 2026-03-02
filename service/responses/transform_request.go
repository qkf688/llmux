package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atopos31/llmio/models"
)

// RequestTransformOptions 请求转换选项。
type RequestTransformOptions struct {
	MapReasoningEffort func(effort string) string
}

// TransformRequest 将 Responses API 请求转换为统一格式。
func TransformRequest(rawBody []byte, options RequestTransformOptions) (*models.UnifiedRequest, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, fmt.Errorf("failed to parse responses request: %w", err)
	}

	unified := &models.UnifiedRequest{
		Model:  req.Model,
		System: req.Instructions,
	}

	if req.Stream != nil {
		unified.Stream = *req.Stream
	}
	if req.MaxOutputTokens != nil {
		unified.MaxTokens = *req.MaxOutputTokens
	}
	if req.Temperature != nil {
		unified.Temperature = req.Temperature
	}
	if req.TopP != nil {
		unified.TopP = req.TopP
	}

	unified.Messages = convertInputToMessages(req.Input)
	if len(req.Tools) > 0 {
		unified.Tools = convertToolsToUnified(req.Tools)
	}

	if effort, ok := req.Metadata["reasoning_effort"].(string); ok && effort != "" {
		if options.MapReasoningEffort != nil {
			mapped := options.MapReasoningEffort(effort)
			unified.ReasoningEffort = &mapped
		} else {
			unified.ReasoningEffort = &effort
		}
	}

	return unified, nil
}

func convertInputToMessages(input ResponsesInput) []models.UnifiedMessage {
	var messages []models.UnifiedMessage

	if input.Text != nil {
		messages = append(messages, models.UnifiedMessage{
			Role:    "user",
			Content: *input.Text,
		})
		return messages
	}

	for _, item := range input.Items {
		switch item.Type {
		case "message", "input_text", "":
			if item.Role != "" {
				msg := models.UnifiedMessage{Role: item.Role}
				if item.Content != nil {
					switch v := item.Content.(type) {
					case string:
						msg.Content = v
					case []interface{}:
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
				messages = append(messages, models.UnifiedMessage{
					Role:    "user",
					Content: *item.Text,
				})
			}

		case "output_text":
			if item.Text != nil {
				messages = append(messages, models.UnifiedMessage{
					Role:    "assistant",
					Content: *item.Text,
				})
			}

		case "function_call":
			if item.Name != nil && item.Arguments != nil {
				toolCall := models.UnifiedToolCall{
					Type: "function",
					Function: models.UnifiedToolCallFunction{
						Name:      *item.Name,
						Arguments: *item.Arguments,
					},
				}
				if item.CallID != nil {
					toolCall.ID = *item.CallID
				} else if item.ID != "" {
					toolCall.ID = item.ID
				}
				messages = append(messages, models.UnifiedMessage{
					Role:      "assistant",
					ToolCalls: []models.UnifiedToolCall{toolCall},
				})
			}

		case "function_call_output":
			if item.Output != nil {
				msg := models.UnifiedMessage{
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

func convertToolsToUnified(tools []ResponsesTool) []models.UnifiedTool {
	var unified []models.UnifiedTool
	for _, tool := range tools {
		if tool.Type == "function" && tool.Function != nil {
			unified = append(unified, models.UnifiedTool{
				Type: "function",
				Function: models.UnifiedFunc{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			})
		}
	}
	return unified
}
