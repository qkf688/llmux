package openai

import (
	"encoding/json"
	"fmt"
)

// ValidateToolCallFunctionNames rejects OpenAI Chat requests that contain tool calls
// missing `tool_calls[].function.name`.
//
// Some upstream models/clients may produce malformed `tool_calls` objects where the
// `function.name` field is omitted. Many providers validate this strictly and will
// return a 400. We validate early to avoid wasting provider retries and to return a
// clearer error to callers.
//
// Best-effort / non-breaking: if the request cannot be parsed, this returns nil
// and allows the request to proceed as before.
func ValidateToolCallFunctionNames(body []byte) error {
	if len(body) == 0 {
		return nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	rawMessages, ok := req["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return nil
	}

	for msgIndex := 0; msgIndex < len(rawMessages); msgIndex++ {
		msgMap, ok := rawMessages[msgIndex].(map[string]any)
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		if role != "assistant" {
			continue
		}

		rawToolCalls, ok := msgMap["tool_calls"].([]any)
		if !ok || len(rawToolCalls) == 0 {
			continue
		}

		for tcIndex := range rawToolCalls {
			tcMap, ok := rawToolCalls[tcIndex].(map[string]any)
			if !ok {
				continue
			}

			// Future-proofing: only validate function tool calls.
			if typ, _ := tcMap["type"].(string); typ != "" && typ != "function" {
				continue
			}

			functionRaw, exists := tcMap["function"]
			if !exists || functionRaw == nil {
				return fmt.Errorf("messages[%d].tool_calls[%d].function is required but missing", msgIndex, tcIndex)
			}

			functionMap, ok := functionRaw.(map[string]any)
			if !ok {
				return fmt.Errorf("messages[%d].tool_calls[%d].function must be an object", msgIndex, tcIndex)
			}

			name, _ := functionMap["name"].(string)
			if name == "" {
				return fmt.Errorf("messages[%d].tool_calls[%d].function.name is required but missing", msgIndex, tcIndex)
			}
		}
	}

	return nil
}
