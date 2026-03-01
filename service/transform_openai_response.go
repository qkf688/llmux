package service

import (
	"encoding/json"
	"github.com/atopos31/llmio/models"
)

func parseOpenAIResponse(body []byte) (*UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &UnifiedResponse{
		ID:      getString(resp, "id"),
		Object:  getString(resp, "object"),
		Created: int64(getFloat(resp, "created")),
		Model:   getString(resp, "model"),
	}

	if choices, ok := resp["choices"].([]interface{}); ok && len(choices) > 0 {
		choice := choices[0].(map[string]interface{})
		msg := choice["message"].(map[string]interface{})

		unified.Choices = []UnifiedChoice{{
			Index: 0,
			Message: &UnifiedMessage{
				Role:      getString(msg, "role"),
				Content:   msg["content"],
				ToolCalls: parseOpenAIToolCalls(msg),
			},
			FinishReason: getString(choice, "finish_reason"),
		}}
	}

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		unified.Usage = &models.Usage{
			PromptTokens:     int64(getFloat(usage, "prompt_tokens")),
			CompletionTokens: int64(getFloat(usage, "completion_tokens")),
			TotalTokens:      int64(getFloat(usage, "total_tokens")),
		}
	}

	return unified, nil
}

func formatOpenAIResponse(unified *UnifiedResponse) ([]byte, error) {
	resp := map[string]interface{}{
		"id":      unified.ID,
		"object":  unified.Object,
		"created": unified.Created,
		"model":   unified.Model,
		"choices": []interface{}{},
	}

	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		msg := map[string]interface{}{
			"role": choice.Message.Role,
		}
		if choice.Message.Content != nil {
			msg["content"] = choice.Message.Content
		}
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls := []interface{}{}
			for _, tc := range choice.Message.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			msg["tool_calls"] = toolCalls
		}

		resp["choices"] = []interface{}{
			map[string]interface{}{
				"index":         choice.Index,
				"message":       msg,
				"finish_reason": choice.FinishReason,
			},
		}
	}

	if unified.Usage != nil {
		resp["usage"] = map[string]interface{}{
			"prompt_tokens":     unified.Usage.PromptTokens,
			"completion_tokens": unified.Usage.CompletionTokens,
			"total_tokens":      unified.Usage.TotalTokens,
		}
	}

	return json.Marshal(resp)
}

func parseOpenAIToolCalls(msgMap map[string]interface{}) []UnifiedToolCall {
	var toolCalls []UnifiedToolCall
	if tcs, ok := msgMap["tool_calls"].([]interface{}); ok {
		for _, tc := range tcs {
			tcMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			funcMap, ok := tcMap["function"].(map[string]interface{})
			if !ok {
				continue
			}

			argsStr := getString(funcMap, "arguments")
			if argsStr == "" {
				argsStr = "{}"
			}

			toolCalls = append(toolCalls, UnifiedToolCall{
				ID:   getString(tcMap, "id"),
				Type: getString(tcMap, "type"),
				Function: UnifiedToolCallFunction{
					Name:      getString(funcMap, "name"),
					Arguments: argsStr,
				},
			})
		}
	}
	return toolCalls
}
