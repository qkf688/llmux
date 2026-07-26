package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atopos31/llmio/models"
)

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
