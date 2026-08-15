package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
)

// ParseResponse 将 Responses API 响应转换为统一格式。
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp responsesResponseDecode
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse responses response: %w", err)
	}

	unified := &models.UnifiedResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.CreatedAt,
		Model:   resp.Model,
	}

	if resp.Error != nil {
		unified.Error = &models.ResponseError{
			Detail: models.ErrorDetail{
				Code:      resp.Error.Code,
				Message:   resp.Error.Message,
				Type:      resp.Error.Type,
				Param:     resp.Error.Param,
				RequestID: resp.Error.RequestID,
			},
		}
		return unified, nil
	}

	if len(resp.Output) > 0 {
		choice := models.UnifiedChoice{
			Index: 0,
		}
		msg := &models.UnifiedMessage{Role: "assistant"}
		var contentParts []models.UnifiedMessageContentPart
		var textContent string
		var toolCalls []models.UnifiedToolCall
		var reasoningContent string
		var toolOutputContent string

		for _, item := range resp.Output {
			switch item.Type {
			case "reasoning":
				if len(item.Summary) > 0 {
					for _, summary := range item.Summary {
						if summary.Type == "summary_text" {
							reasoningContent += summary.Text
						}
					}
				}
			case "output_text":
				if item.Text != nil {
					textContent += *item.Text
				}
			case "message":
				if item.Role != "assistant" {
					continue
				}
				if parsed, ok := parseResponsesMessageContent(item.Content); ok {
					switch v := parsed.(type) {
					case string:
						textContent += v
					case []models.UnifiedMessageContentPart:
						contentParts = append(contentParts, v...)
					}
				}
			case "function_call":
				if item.Name != nil && item.Arguments != nil {
					tc := models.UnifiedToolCall{
						Type: "function",
						Function: models.UnifiedToolCallFunction{
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
			case "function_call_output":
				if item.Output != nil {
					switch v := item.Output.(type) {
					case string:
						if strings.TrimSpace(v) != "" {
							if toolOutputContent != "" {
								toolOutputContent += "\n\n"
							}
							toolOutputContent += v
						}
					case []interface{}:
						if parts, ok := parsePartsToUnifiedContent(v); ok {
							switch p := parts.(type) {
							case string:
								if strings.TrimSpace(p) != "" {
									if toolOutputContent != "" {
										toolOutputContent += "\n\n"
									}
									toolOutputContent += p
								}
							case []models.UnifiedMessageContentPart:
								contentParts = append(contentParts, p...)
							}
						}
					}
				}
			case "error":
				if item.Error != nil {
					unified.Error = &models.ResponseError{
						Detail: models.ErrorDetail{
							Code:      item.Error.Code,
							Message:   item.Error.Message,
							Type:      item.Error.Type,
							Param:     item.Error.Param,
							RequestID: item.Error.RequestID,
						},
					}
				}
			}
		}

		if reasoningContent != "" {
			msg.SetReasoningContent(reasoningContent)
		}

		if toolOutputContent != "" {
			toolOutputText := toolOutputContent
			contentParts = append(contentParts, models.UnifiedMessageContentPart{
				Type: "text",
				Text: &toolOutputText,
			})
		}

		if len(contentParts) > 0 {
			if textContent != "" {
				txt := textContent
				contentParts = append(contentParts, models.UnifiedMessageContentPart{
					Type: "text",
					Text: &txt,
				})
			}
			if reasoningContent != "" {
				prefix := reasoningContent + "\n\n---\n\n"
				contentParts = append([]models.UnifiedMessageContentPart{{
					Type: "text",
					Text: &prefix,
				}}, contentParts...)
			}
			msg.Content = contentParts
		} else if textContent != "" || reasoningContent != "" {
			if reasoningContent != "" && textContent != "" {
				textContent = reasoningContent + "\n\n---\n\n" + textContent
			} else if reasoningContent != "" {
				textContent = reasoningContent
			}
			if textContent != "" {
				msg.Content = textContent
			}
		}

		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}

		choice.Message = msg

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

		unified.Choices = []models.UnifiedChoice{choice}
	}

	if resp.Usage != nil {
		unified.Usage = &models.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.InputTokenDetails != nil {
			unified.Usage.PromptTokensDetails.CachedTokens = resp.Usage.InputTokenDetails.CachedTokens
		}
		if resp.Usage.OutputTokenDetails != nil {
			unified.Usage.CompletionTokensDetails.ReasoningTokens = resp.Usage.OutputTokenDetails.ReasoningTokens
		}
	}

	return unified, nil
}
