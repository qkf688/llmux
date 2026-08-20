package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
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
	if req.Input.Text != nil {
		wasArray := false
		unified.TransformOptions.ArrayInputs = &wasArray
	} else if req.Input.Items != nil {
		wasArray := true
		unified.TransformOptions.ArrayInputs = &wasArray
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

	var extractedSystem string
	unified.Messages, extractedSystem = convertInputToMessages(req.Input)
	if extractedSystem != "" {
		if unified.System != "" {
			unified.System = unified.System + "\n\n" + extractedSystem
		} else {
			unified.System = extractedSystem
		}
	}
	if len(req.Tools) > 0 {
		unified.Tools = convertToolsToUnified(req.Tools)
	}

	if req.ToolChoice != nil {
		unified.ToolChoice = &models.UnifiedToolChoice{}
		if req.ToolChoice.StringValue != nil {
			unified.ToolChoice.StringValue = req.ToolChoice.StringValue
		} else if req.ToolChoice.ObjectValue != nil && req.ToolChoice.ObjectValue.Function != nil {
			unified.ToolChoice.ObjectValue = &models.UnifiedToolChoiceObject{
				Type: req.ToolChoice.ObjectValue.Type,
				Function: &models.UnifiedToolChoiceFunction{
					Name: req.ToolChoice.ObjectValue.Function.Name,
				},
			}
		}
	}

	if req.Text != nil && req.Text.Format != nil && req.Text.Format.Type != "" {
		unified.ResponseFormat = &models.UnifiedResponseFormat{
			Type:       req.Text.Format.Type,
			JSONSchema: req.Text.Format.JSONSchema,
		}
	}

	if req.Metadata != nil {
		metadata := make(map[string]string, 0)
		for k, v := range req.Metadata {
			if s, ok := v.(string); ok && s != "" {
				metadata[k] = s
			}
		}
		if len(metadata) > 0 {
			unified.Metadata = metadata
		}
	}

	// effort 只认官方字段 reasoning.effort；metadata 是客户端自由 KV 标签，不是控制通道。
	effort := ""
	if req.Reasoning != nil && req.Reasoning.Effort != nil {
		effort = *req.Reasoning.Effort
	}
	if effort != "" {
		if options.MapReasoningEffort != nil {
			mapped := options.MapReasoningEffort(effort)
			unified.ReasoningEffort = &mapped
		} else {
			unified.ReasoningEffort = &effort
		}
	}
	if req.Reasoning != nil && req.Reasoning.MaxTokens != nil {
		unified.ReasoningBudget = req.Reasoning.MaxTokens
	}

	// 缓存与安全相关：与 TransformFromUnified 对称
	unified.ServiceTier = req.ServiceTier
	unified.SafetyIdentifier = req.SafetyIdentifier
	unified.PromptCacheKey = req.PromptCacheKey

	return unified, nil
}

func convertInputToMessages(input ResponsesInput) ([]models.UnifiedMessage, string) {
	var messages []models.UnifiedMessage
	var systemParts []string

	if input.Text != nil {
		messages = append(messages, models.UnifiedMessage{
			Role:    "user",
			Content: *input.Text,
		})
		return messages, ""
	}

	nonSystemCount := 0
	for _, item := range input.Items {
		if item.Role == "" {
			continue
		}
		if item.Role != "system" && item.Role != "developer" {
			nonSystemCount++
		}
	}
	shouldExtractSystem := nonSystemCount > 0

	for _, item := range input.Items {
		switch item.Type {
		case "message", "input_text", "":
			if item.Role != "" {
				if (item.Role == "system" || item.Role == "developer") && shouldExtractSystem {
					systemContent := ""
					if item.Content != nil {
						if s, ok := item.Content.(string); ok {
							systemContent = s
						} else if parts, ok := item.Content.([]interface{}); ok {
							systemContent = extractTextFromParts(parts)
						}
					} else if item.Text != nil {
						systemContent = *item.Text
					}
					if systemContent != "" {
						systemParts = append(systemParts, systemContent)
					}
					continue
				}

				msg := models.UnifiedMessage{Role: item.Role}
				if item.Content != nil {
					switch v := item.Content.(type) {
					case string:
						msg.Content = v
					case []interface{}:
						if content, ok := parsePartsToUnifiedContent(v); ok {
							msg.Content = content
						}
					default:
						msg.Content = item.Content
					}
				} else if item.Text != nil {
					msg.Content = *item.Text
				}

				if item.Role == "tool" && item.CallID != nil {
					msg.ToolCallID = *item.CallID
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
					Role: "tool",
				}
				switch v := item.Output.(type) {
				case string:
					msg.Content = v
				case []interface{}:
					if content, ok := parsePartsToUnifiedContent(v); ok {
						msg.Content = content
					} else {
						// 数组形式但解析失败，降级为空串避免整条丢弃
						msg.Content = ""
					}
				default:
					// 未知类型降级为空串，避免泄漏内部结构
					msg.Content = ""
				}
				if item.CallID != nil {
					msg.ToolCallID = *item.CallID
				}
				messages = append(messages, msg)
			}
		}
	}

	return messages, strings.Join(systemParts, "\n\n")
}

func convertToolsToUnified(tools []ResponsesTool) []models.UnifiedTool {
	var unified []models.UnifiedTool
	for _, tool := range tools {
		if tool.Type == "function" && tool.Name != "" {
			unified = append(unified, models.UnifiedTool{
				Type: "function",
				Function: models.UnifiedFunc{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}
	return unified
}

func extractTextFromParts(parts []interface{}) string {
	var textParts []string
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		if partType != "input_text" && partType != "output_text" && partType != "text" {
			continue
		}
		if text, ok := partMap["text"].(string); ok {
			textParts = append(textParts, text)
		}
	}
	return strings.Join(textParts, "")
}

func parsePartsToUnifiedContent(parts []interface{}) (any, bool) {
	textOnly := true
	textParts := make([]string, 0, len(parts))
	unifiedParts := make([]models.UnifiedMessageContentPart, 0, len(parts))

	for _, raw := range parts {
		partMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		partType, _ := partMap["type"].(string)
		switch partType {
		case "input_text", "output_text", "text":
			if text, ok := partMap["text"].(string); ok {
				textParts = append(textParts, text)
				txt := text
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
					Type: "text",
					Text: &txt,
				})
			}

		case "input_image", "image_url":
			textOnly = false

			url := ""
			detail := (*string)(nil)

			if v, ok := partMap["image_url"]; ok && v != nil {
				switch vv := v.(type) {
				case string:
					url = vv
				case map[string]interface{}:
					if s, ok := vv["url"].(string); ok {
						url = s
					}
					if s, ok := vv["detail"].(string); ok {
						detail = &s
					}
				}
			}
			if url == "" {
				if s, ok := partMap["url"].(string); ok {
					url = s
				}
			}
			if detail == nil {
				if s, ok := partMap["detail"].(string); ok {
					detail = &s
				}
			}

			if url != "" {
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
					Type: "image_url",
					ImageURL: &models.UnifiedImageURL{
						URL:    url,
						Detail: detail,
					},
				})
			}

		default:
			// Unknown part types should keep array form to avoid dropping non-text inputs.
			if partType != "" {
				textOnly = false
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{Type: partType})
			}
		}
	}

	if len(unifiedParts) == 0 {
		return nil, false
	}

	if textOnly {
		return strings.Join(textParts, ""), true
	}

	return unifiedParts, true
}
