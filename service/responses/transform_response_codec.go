package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atopos31/llmio/models"
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
				if item.Output != nil && strings.TrimSpace(*item.Output) != "" {
					if toolOutputContent != "" {
						toolOutputContent += "\n\n"
					}
					toolOutputContent += *item.Output
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
	}

	return unified, nil
}

// FormatResponse 将统一格式转换为 Responses API 响应。
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	if unified.Error != nil && len(unified.Choices) == 0 {
		return json.Marshal(struct {
			Error *ResponsesError `json:"error"`
		}{
			Error: &ResponsesError{
				Type:      unified.Error.Detail.Type,
				Code:      unified.Error.Detail.Code,
				Message:   unified.Error.Detail.Message,
				Param:     unified.Error.Detail.Param,
				RequestID: unified.Error.Detail.RequestID,
			},
		})
	}

	resp := ResponsesResponse{
		Object:    "response",
		ID:        unified.ID,
		Model:     unified.Model,
		CreatedAt: unified.Created,
	}

	if len(unified.Choices) > 0 {
		var output []ResponsesItem

		for _, choice := range unified.Choices {
			if choice.Message == nil {
				continue
			}

			if choice.Message.Role == "tool" {
				if choice.Message.ToolCallID == "" {
					continue
				}
				callID := choice.Message.ToolCallID
				out := choice.Message.GetContentAsString()
				if strings.TrimSpace(out) == "" {
					continue
				}
				outputText := out
				output = append(output, ResponsesItem{
					Type:   "function_call_output",
					CallID: &callID,
					Output: &outputText,
				})
				continue
			}

			for _, tc := range choice.Message.ToolCalls {
				callID := tc.ID
				var callIDPtr *string
				if callID != "" {
					callIDPtr = &callID
				}
				// Provide an `id` with the expected prefix for better client compatibility.
				// `call_id` remains the stable identifier used to correlate tool outputs.
				fcID := ""
				if callID != "" {
					fcID = "fc_" + callID
				}
				name := tc.Function.Name
				arguments := tc.Function.Arguments
				output = append(output, ResponsesItem{
					Type:      "function_call",
					ID:        fcID,
					CallID:    callIDPtr,
					Name:      &name,
					Arguments: &arguments,
				})
			}

			if contentItems := unifiedMessageContentToResponses(choice.Message.Content); len(contentItems) > 0 {
				status := "completed"
				msgID := fmt.Sprintf("msg_%s", unified.ID)
				output = append(output, ResponsesItem{
					ID:      msgID,
					Type:    "message",
					Role:    "assistant",
					Status:  &status,
					Content: contentItems,
				})
			}
		}

		resp.Output = output

		status := "completed"
		switch unified.Choices[0].FinishReason {
		case "stop", "tool_calls":
			status = "completed"
		case "length":
			status = "incomplete"
		case "error":
			status = "failed"
		}
		resp.Status = &status
	}

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
		if unified.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
			resp.Usage.OutputTokenDetails = &ResponsesOutputTokenDetails{
				ReasoningTokens: unified.Usage.CompletionTokensDetails.ReasoningTokens,
			}
		}
	}

	return json.Marshal(resp)
}

func unifiedMessageContentToResponses(content any) []responsesContentItem {
	switch v := content.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		text := v
		return []responsesContentItem{{
			Type:        "output_text",
			Text:        &text,
			Annotations: []ResponsesAnnotation{},
		}}
	case []models.UnifiedMessageContentPart:
		return unifiedPartsToResponsesContentItems("assistant", v)
	default:
		return nil
	}
}

func unifiedPartsToResponsesContentItems(role string, parts []models.UnifiedMessageContentPart) []responsesContentItem {
	if len(parts) == 0 {
		return nil
	}
	out := make([]responsesContentItem, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil || strings.TrimSpace(*part.Text) == "" {
				continue
			}
			partType := "input_text"
			if role == "assistant" {
				partType = "output_text"
			}
			txt := *part.Text
			out = append(out, responsesContentItem{
				Type:        partType,
				Text:        &txt,
				Annotations: []ResponsesAnnotation{},
			})
		case "image_url":
			if part.ImageURL == nil || part.ImageURL.URL == "" {
				continue
			}
			url := part.ImageURL.URL
			out = append(out, responsesContentItem{
				Type:     "image_url",
				ImageURL: &url,
				Detail:   part.ImageURL.Detail,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseResponsesMessageContent(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if strings.TrimSpace(str) == "" {
			return nil, false
		}
		return str, true
	}

	var parts []responsesContentItemDecode
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, false
	}
	return responsesContentItemsToUnified(parts)
}

func responsesContentItemsToUnified(parts []responsesContentItemDecode) (any, bool) {
	if len(parts) == 0 {
		return nil, false
	}

	textOnly := true
	textParts := make([]string, 0, len(parts))
	unifiedParts := make([]models.UnifiedMessageContentPart, 0, len(parts))

	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			if part.Text == nil {
				continue
			}
			txt := *part.Text
			textParts = append(textParts, txt)
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
				Type: "text",
				Text: &txt,
			})
		case "input_image", "output_image", "image_url":
			textOnly = false
			url, detail := decodeResponsesImageURL(part)
			if url == "" {
				continue
			}
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
				Type: "image_url",
				ImageURL: &models.UnifiedImageURL{
					URL:    url,
					Detail: detail,
				},
			})
		default:
			if part.Type == "" {
				continue
			}
			textOnly = false
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{Type: part.Type})
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

func decodeResponsesImageURL(part responsesContentItemDecode) (string, *string) {
	if part.ImageURL != nil && len(*part.ImageURL) > 0 {
		var urlStr string
		if err := json.Unmarshal(*part.ImageURL, &urlStr); err == nil {
			if urlStr != "" {
				return urlStr, part.Detail
			}
		}

		var obj responsesImageURLDecode
		if err := json.Unmarshal(*part.ImageURL, &obj); err == nil && obj.URL != "" {
			if obj.Detail != nil && *obj.Detail != "" {
				return obj.URL, obj.Detail
			}
			return obj.URL, part.Detail
		}
	}

	if part.URL != nil && *part.URL != "" {
		return *part.URL, part.Detail
	}

	return "", part.Detail
}

type responsesResponseDecode struct {
	Object    string                `json:"object"`
	ID        string                `json:"id"`
	Model     string                `json:"model"`
	CreatedAt int64                 `json:"created_at"`
	Output    []responsesItemDecode `json:"output"`
	Status    *string               `json:"status,omitempty"`
	Usage     *ResponsesUsage       `json:"usage,omitempty"`
	Error     *ResponsesError       `json:"error,omitempty"`
}

type responsesItemDecode struct {
	ID        string                      `json:"id,omitempty"`
	Type      string                      `json:"type"`
	Role      string                      `json:"role,omitempty"`
	Text      *string                     `json:"text,omitempty"`
	Name      *string                     `json:"name,omitempty"`
	Arguments *string                     `json:"arguments,omitempty"`
	CallID    *string                     `json:"call_id,omitempty"`
	Output    *string                     `json:"output,omitempty"`
	Summary   []ResponsesReasoningSummary `json:"summary,omitempty"`
	Content   json.RawMessage             `json:"content,omitempty"`
	Error     *ResponsesError             `json:"error,omitempty"`
}

type responsesContentItem struct {
	Type        string                `json:"type"`
	Text        *string               `json:"text,omitempty"`
	Annotations []ResponsesAnnotation `json:"annotations,omitempty"`
	ImageURL    *string               `json:"image_url,omitempty"`
	Detail      *string               `json:"detail,omitempty"`
}

type responsesContentItemDecode struct {
	Type     string           `json:"type"`
	Text     *string          `json:"text,omitempty"`
	ImageURL *json.RawMessage `json:"image_url,omitempty"`
	URL      *string          `json:"url,omitempty"`
	Detail   *string          `json:"detail,omitempty"`
}

type responsesImageURLDecode struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}
