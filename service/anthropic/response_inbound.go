package anthropic

import (
	"encoding/json"
	"time"
)

// ParseResponse 将 Anthropic 响应格式转换为统一格式。
func ParseResponse(body []byte) (*UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &UnifiedResponse{
		ID:      getString(resp, "id"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   getString(resp, "model"),
	}

	textContent := ""
	toolCalls := make([]UnifiedToolCall, 0)

	if content, ok := asSlice(resp["content"]); ok {
		for _, item := range content {
			itemMap, ok := asMap(item)
			if !ok {
				continue
			}

			switch getString(itemMap, "type") {
			case "text":
				textContent += getString(itemMap, "text")
			case "tool_use":
				args, _ := json.Marshal(itemMap["input"])
				toolCalls = append(toolCalls, UnifiedToolCall{
					ID:   getString(itemMap, "id"),
					Type: "function",
					Function: UnifiedToolCallFunction{
						Name:      getString(itemMap, "name"),
						Arguments: string(args),
					},
				})
			}
		}
	}

	finishReason := getString(resp, "stop_reason")
	if finishReason == "end_turn" {
		finishReason = "stop"
	} else if finishReason == "tool_use" {
		finishReason = "tool_calls"
	}

	unified.Choices = []UnifiedChoice{{
		Index: 0,
		Message: &UnifiedMessage{
			Role:      "assistant",
			Content:   textContent,
			ToolCalls: toolCalls,
		},
		FinishReason: finishReason,
	}}

	if usage, ok := asMap(resp["usage"]); ok {
		unified.Usage = &Usage{
			PromptTokens:     int64(getFloat(usage, "input_tokens")),
			CompletionTokens: int64(getFloat(usage, "output_tokens")),
			TotalTokens:      int64(getFloat(usage, "input_tokens") + getFloat(usage, "output_tokens")),
		}
	}

	return unified, nil
}
