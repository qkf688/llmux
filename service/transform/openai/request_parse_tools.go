package openai

import (
	"encoding/json"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

func parseOpenAIChatTools(tools shared.RawArray) []models.UnifiedTool {
	var unified []models.UnifiedTool
	for _, raw := range tools {
		var tool openAIChatTool
		if !decodeOpenAIChatObject(raw, &tool) {
			continue
		}
		var fn openAIChatFunction
		if !decodeOpenAIChatObject(tool.Function, &fn) {
			continue
		}
		unified = append(unified, models.UnifiedTool{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        fn.Name.Value,
				Description: fn.Description.Value,
				Parameters:  shared.RawJSONValue(fn.Parameters),
			},
		})
	}
	return unified
}

func parseOpenAIChatToolCalls(toolCalls shared.RawArray) []models.UnifiedToolCall {
	var unified []models.UnifiedToolCall
	for _, raw := range toolCalls {
		var toolCall openAIChatToolCall
		if !decodeOpenAIChatObject(raw, &toolCall) {
			continue
		}
		var fn openAIChatToolCallFunction
		if !decodeOpenAIChatObject(toolCall.Function, &fn) {
			continue
		}
		unified = append(unified, models.UnifiedToolCall{
			ID:   toolCall.ID.Value,
			Type: toolCall.Type.Value,
			Function: models.UnifiedToolCallFunction{
				Name:      fn.Name.Value,
				Arguments: normalizeOpenAIChatToolCallArguments(fn.Arguments),
			},
		})
	}
	return unified
}

func normalizeOpenAIChatToolCallArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if str != "" {
			return str
		}
		return "{}"
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if b, err := json.Marshal(obj); err == nil && len(b) > 0 && string(b) != "null" {
			return string(b)
		}
	}
	var arr []interface{}
	if err := json.Unmarshal(raw, &arr); err == nil {
		if b, err := json.Marshal(arr); err == nil && len(b) > 0 && string(b) != "null" {
			return string(b)
		}
	}
	return "{}"
}
