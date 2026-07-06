package anthropic

import (
	"encoding/json"
	"fmt"
	"github.com/atopos31/llmio/models"

	"github.com/atopos31/llmio/common/maputil"
)

// ParseResponse 将 Anthropic 响应格式转换为统一格式。
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &models.UnifiedResponse{
		ID:      maputil.String(resp, "id"),
		Object:  "chat.completion",
		Created: 0,
		Model:   maputil.String(resp, "model"),
	}

	finishReason := maputil.String(resp, "stop_reason")
	switch finishReason {
	case "end_turn", "stop_sequence":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	content, _ := parseMessageContentAndToolResults(resp["content"])
	toolCalls := parseToolCalls(resp["content"])

	reasoningText, reasoningSig := extractThinking(resp["content"])
	if reasoningText != "" {
		switch v := content.(type) {
		case string:
			if v != "" {
				content = reasoningText + "\n\n---\n\n" + v
			} else {
				content = reasoningText
			}
		case []models.UnifiedMessageContentPart:
			prefix := reasoningText
			if len(v) > 0 {
				prefix += "\n\n---\n\n"
			}
			v2 := make([]models.UnifiedMessageContentPart, 0, len(v)+1)
			v2 = append(v2, models.UnifiedMessageContentPart{Type: "text", Text: &prefix})
			v2 = append(v2, v...)
			content = v2
		case nil:
			content = reasoningText
		default:
			// Keep behavior: best-effort stringify unknown content shapes.
			content = fmt.Sprintf("%v", v)
		}
	}

	message := &models.UnifiedMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
	if reasoningText != "" {
		message.ReasoningContent = &reasoningText
	}
	if reasoningSig != "" {
		message.ReasoningSignature = &reasoningSig
	}

	unified.Choices = []models.UnifiedChoice{{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}}

	if usage, ok := asMap(resp["usage"]); ok {
		unified.Usage = &models.Usage{
			PromptTokens:     int64(maputil.Float64(usage, "input_tokens")),
			CompletionTokens: int64(maputil.Float64(usage, "output_tokens")),
			TotalTokens:      int64(maputil.Float64(usage, "input_tokens") + maputil.Float64(usage, "output_tokens")),
		}
	}

	return unified, nil
}

func extractThinking(raw interface{}) (thinking string, signature string) {
	items, ok := asSlice(raw)
	if !ok {
		return "", ""
	}

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}
		if maputil.String(itemMap, "type") != "thinking" {
			continue
		}
		thinking = maputil.String(itemMap, "thinking")
		signature = maputil.String(itemMap, "signature")
		if thinking != "" || signature != "" {
			return thinking, signature
		}
	}

	return "", ""
}
