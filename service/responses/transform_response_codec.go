package responses

import (
	"encoding/json"
	"fmt"

	"github.com/atopos31/llmio/models"
)

// ParseResponse 将 Responses API 响应转换为统一格式。
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp ResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse responses response: %w", err)
	}

	unified := &models.UnifiedResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.CreatedAt,
		Model:   resp.Model,
	}

	if len(resp.Output) > 0 {
		choice := models.UnifiedChoice{
			Index: 0,
		}
		msg := &models.UnifiedMessage{Role: "assistant"}
		var textContent string
		var toolCalls []models.UnifiedToolCall
		var reasoningContent string

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
			}
		}

		if reasoningContent != "" {
			if textContent != "" {
				textContent = reasoningContent + "\n\n---\n\n" + textContent
			} else {
				textContent = reasoningContent
			}
		}
		if textContent != "" {
			msg.Content = textContent
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
	resp := ResponsesResponse{
		Object:    "response",
		ID:        unified.ID,
		Model:     unified.Model,
		CreatedAt: unified.Created,
	}

	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		var output []ResponsesItem

		if choice.Message != nil {
			for _, tc := range choice.Message.ToolCalls {
				output = append(output, ResponsesItem{
					Type:      "function_call",
					ID:        tc.ID,
					CallID:    &tc.ID,
					Name:      &tc.Function.Name,
					Arguments: &tc.Function.Arguments,
				})
			}

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
