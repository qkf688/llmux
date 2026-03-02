package anthropic

import (
	"encoding/json"
	"errors"
)

// FormatResponse 将统一响应格式转换为 Anthropic 响应格式。
func FormatResponse(unified *UnifiedResponse) ([]byte, error) {
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
			if textStr, ok := choice.Message.Content.(string); ok && textStr != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": textStr,
				})
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
		if stopReason == "stop" {
			stopReason = "end_turn"
		} else if stopReason == "tool_calls" {
			stopReason = "tool_use"
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
