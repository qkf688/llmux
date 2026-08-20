package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
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

		for choiceIdx, choice := range unified.Choices {
			if choice.Message == nil {
				continue
			}

			if choice.Message.Role == "tool" {
				if choice.Message.ToolCallID == "" {
					continue
				}
				callID := choice.Message.ToolCallID
				item := ResponsesItem{
					Type:   "function_call_output",
					CallID: &callID,
				}
				switch content := choice.Message.Content.(type) {
				case string:
					if strings.TrimSpace(content) == "" {
						continue
					}
					item.Output = content
				case []models.UnifiedMessageContentPart:
					arr := unifiedPartsToToolOutput(content)
					if arr == nil {
						// 无有效块，降级为占位符避免空 output
						fallback := choice.Message.GetContentAsStringWithPlaceholders()
						if strings.TrimSpace(fallback) == "" {
							continue
						}
						item.Output = fallback
					} else {
						item.Output = arr
					}
				default:
					fallback := choice.Message.GetContentAsStringWithPlaceholders()
					if strings.TrimSpace(fallback) == "" {
						continue
					}
					item.Output = fallback
				}
				output = append(output, item)
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
				// item id 在 output 数组内必须唯一：OpenAI Responses 的 msg_ 是 message 级
				// 标识，客户端把 output 回灌成下一轮 input 时同 id 会被判 "Duplicate item
				// found with id msg_..." 而 400。unified.ID 是整个响应的 id，多 choice 时
				// 对每个 choice 都取它就会撞车，故第二个及之后的 choice 追加序号后缀。
				// 首个不加后缀：单 choice 是绝对主流形态，保持既有 id 稳定。
				msgID := fmt.Sprintf("msg_%s", unified.ID)
				if choiceIdx > 0 {
					msgID = fmt.Sprintf("%s_%d", msgID, choiceIdx)
				}
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
