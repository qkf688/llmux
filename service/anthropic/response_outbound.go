package anthropic

import (
	"encoding/json"
	"errors"
	"github.com/atopos31/llmio/models"
	"strings"
)

// FormatResponse 将统一响应格式转换为 Anthropic 响应格式。
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	if unified == nil {
		return nil, errors.New("unified response cannot be nil")
	}

	resp := map[string]interface{}{
		"id":      unified.ID,
		"type":    "message",
		"role":    "assistant",
		"model":   unified.Model,
		"content": []interface{}{},
	}

	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		content := []interface{}{}

		if choice.Message != nil {
			reasoning := choice.Message.GetReasoningContent()
			adjustedContent := stripReasoningPrefix(choice.Message.Content, reasoning)

			if reasoning != "" {
				thinking := map[string]interface{}{
					"type":     "thinking",
					"thinking": reasoning,
				}
				if choice.Message.ReasoningSignature != nil && *choice.Message.ReasoningSignature != "" {
					thinking["signature"] = *choice.Message.ReasoningSignature
				}
				content = append(content, thinking)
			}

			if parts, ok := adjustedContent.([]models.UnifiedMessageContentPart); ok {
				content = append(content, buildContentParts(parts)...)
			} else if textStr, ok := adjustedContent.(string); ok && textStr != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": textStr,
				})
			} else if adjustedContent != nil {
				// Best-effort: keep unknown content shapes as a JSON-encoded text block.
				if b, err := json.Marshal(adjustedContent); err == nil && len(b) > 0 {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": string(b),
					})
				}
			}

			for _, toolCall := range choice.Message.ToolCalls {
				var input map[string]interface{}
				if toolCall.Function.Arguments != "" {
					_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
				}

				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    toolCall.ID,
					"name":  toolCall.Function.Name,
					"input": input,
				})
			}
		}

		resp["content"] = content

		stopReason := choice.FinishReason
		switch stopReason {
		case "stop":
			stopReason = "end_turn"
		case "tool_calls":
			stopReason = "tool_use"
		case "length":
			stopReason = "max_tokens"
		}
		resp["stop_reason"] = stopReason
	}

	if unified.Usage != nil {
		resp["usage"] = map[string]interface{}{
			"input_tokens":  unified.Usage.PromptTokens,
			"output_tokens": unified.Usage.CompletionTokens,
		}
	}

	return json.Marshal(resp)
}

func stripReasoningPrefix(content interface{}, reasoning string) interface{} {
	if content == nil || reasoning == "" {
		return content
	}

	prefix := reasoning + "\n\n---\n\n"

	switch v := content.(type) {
	case string:
		if v == reasoning {
			return ""
		}
		if strings.HasPrefix(v, prefix) {
			return strings.TrimPrefix(v, prefix)
		}
		return v

	case []models.UnifiedMessageContentPart:
		if len(v) == 0 {
			return v
		}
		first := v[0]
		if first.Type != "text" || first.Text == nil {
			return v
		}
		if *first.Text == reasoning || *first.Text == prefix {
			return v[1:]
		}
		return v

	default:
		return v
	}
}
