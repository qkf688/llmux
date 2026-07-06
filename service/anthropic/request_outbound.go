package anthropic

import (
	"encoding/json"
	"errors"
	"github.com/atopos31/llmio/models"

	"github.com/atopos31/llmio/common"
)

// TransformFromUnified 将统一请求格式转换为 Anthropic 请求格式。
func TransformFromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	if unified == nil {
		return nil, errors.New("unified request cannot be nil")
	}

	unified = unified.SanitizedForProvider()

	req := map[string]interface{}{
		"model":    unified.Model,
		"messages": []interface{}{},
		"stream":   unified.Stream,
	}

	if unified.MaxTokens > 0 {
		req["max_tokens"] = unified.MaxTokens
	} else {
		req["max_tokens"] = 8192
	}
	if unified.Temperature != nil {
		req["temperature"] = *unified.Temperature
	}
	if unified.TopP != nil {
		req["top_p"] = *unified.TopP
	}
	if system := buildSystemValue(unified); system != nil {
		req["system"] = system
	}

	req["messages"] = buildMessages(unified, req)

	if len(unified.Tools) > 0 {
		req["tools"] = buildTools(unified.Tools)
	}

	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop_sequences"] = []string{*unified.Stop.Single}
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop_sequences"] = unified.Stop.Multiple
		}
	}

	if len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}

	if unified.ToolChoice != nil {
		if unified.ToolChoice.StringValue != nil {
			req["tool_choice"] = map[string]interface{}{
				"type": *unified.ToolChoice.StringValue,
			}
		} else if unified.ToolChoice.ObjectValue != nil && unified.ToolChoice.ObjectValue.Function != nil {
			req["tool_choice"] = map[string]interface{}{
				"type": "tool",
				"name": unified.ToolChoice.ObjectValue.Function.Name,
			}
		}
	}

	if unified.ReasoningEffort != nil || unified.ReasoningBudget != nil {
		thinking := map[string]interface{}{
			"type": "enabled",
		}

		if unified.ReasoningBudget != nil {
			thinking["budget_tokens"] = *unified.ReasoningBudget
		} else if unified.ReasoningEffort != nil {
			thinking["budget_tokens"] = reasoningEffortToThinkingBudget(*unified.ReasoningEffort)
		}

		req["thinking"] = thinking
	}

	return json.Marshal(req)
}

func buildMessages(unified *models.UnifiedRequest, req map[string]interface{}) []interface{} {
	messages := make([]interface{}, 0, len(unified.Messages))

	nonSystemCount := 0
	for _, msg := range unified.Messages {
		if msg.Role != "system" {
			nonSystemCount++
		}
	}
	shouldExtractSystem := nonSystemCount > 0

	for _, msg := range unified.Messages {
		if msg.Role == "system" && shouldExtractSystem {
			if parts, ok := msg.Content.([]models.UnifiedMessageContentPart); ok {
				appendSystemParts(req, parts)
			} else if content, ok := msg.Content.(string); ok {
				appendSystemText(req, content)
			}
			continue
		}

		if msg.Role == "tool" {
			contentStr := ""
			if content, ok := msg.Content.(string); ok {
				contentStr = content
			}

			messages = append(messages, map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": msg.ToolCallID,
						"content":     contentStr,
					},
				},
			})
			continue
		}

		msgMap := map[string]interface{}{
			"role": msg.Role,
		}

		if msg.CacheControl != nil {
			msgMap["cache_control"] = map[string]interface{}{
				"type": msg.CacheControl.Type,
			}
		}

		if msg.Content != nil {
			if parts, ok := msg.Content.([]models.UnifiedMessageContentPart); ok {
				contentArray := buildContentParts(parts)
				if len(contentArray) > 0 {
					msgMap["content"] = contentArray
				}
			} else {
				msgMap["content"] = msg.Content
			}
		}

		if len(msg.ToolCalls) > 0 {
			msgMap["content"] = buildToolUseContent(msg)
		}

		messages = append(messages, msgMap)
	}

	return messages
}

func buildContentParts(parts []models.UnifiedMessageContentPart) []interface{} {
	contentArray := make([]interface{}, 0, len(parts))

	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil {
				continue
			}
			textMap := map[string]interface{}{
				"type": "text",
				"text": *part.Text,
			}
			if part.CacheControl != nil {
				textMap["cache_control"] = map[string]interface{}{
					"type": part.CacheControl.Type,
				}
			}
			contentArray = append(contentArray, textMap)
		case "image_url":
			if part.ImageURL == nil {
				continue
			}

			imgMap := map[string]interface{}{
				"type": "image",
			}
			if mediaType, data, ok := common.ParseBase64DataURL(part.ImageURL.URL); ok {
				imgMap["source"] = map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				}
			} else {
				imgMap["source"] = map[string]interface{}{
					"type": "url",
					"url":  part.ImageURL.URL,
				}
			}

			if part.CacheControl != nil {
				imgMap["cache_control"] = map[string]interface{}{
					"type": part.CacheControl.Type,
				}
			}
			contentArray = append(contentArray, imgMap)
		}
	}

	return contentArray
}

func buildToolUseContent(msg models.UnifiedMessage) []interface{} {
	contentArray := make([]interface{}, 0, len(msg.ToolCalls)+1)

	if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
		contentArray = append(contentArray, map[string]interface{}{
			"type": "text",
			"text": contentStr,
		})
	}

	for _, toolCall := range msg.ToolCalls {
		args := map[string]interface{}{}
		if toolCall.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{}
			}
		}

		contentArray = append(contentArray, map[string]interface{}{
			"type":  "tool_use",
			"id":    toolCall.ID,
			"name":  toolCall.Function.Name,
			"input": args,
		})
	}

	return contentArray
}

func buildTools(tools []models.UnifiedTool) []interface{} {
	result := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		toolMap := map[string]interface{}{
			"name":         tool.Function.Name,
			"description":  tool.Function.Description,
			"input_schema": tool.Function.Parameters,
		}

		if tool.CacheControl != nil {
			toolMap["cache_control"] = map[string]interface{}{
				"type": tool.CacheControl.Type,
			}
		}

		result = append(result, toolMap)
	}
	return result
}
