package openai

import (
	"encoding/json"
	"fmt"
)

// StripEmptyResponsesInputNames removes empty-string `name` fields from Responses API `input` items.
//
// OpenAI rejects empty strings for `input[].name`. For normal role-based message items, `name` is optional
// and should be omitted when unset. For function_call items, `name` is required, so we return an error
// instead of silently dropping it.
func StripEmptyResponsesInputNames(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		// Best-effort: if we can't parse, keep behavior non-breaking.
		return body, false, nil
	}

	rawInput, ok := req["input"]
	if !ok || rawInput == nil {
		return body, false, nil
	}

	inputItems, ok := rawInput.([]any)
	if !ok || len(inputItems) == 0 {
		return body, false, nil
	}

	changed := false
	for idx, rawItem := range inputItems {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		name, hasName := itemMap["name"].(string)
		if !hasName || name != "" {
			continue
		}

		itemType, _ := itemMap["type"].(string)
		if itemType == "function_call" {
			return body, false, fmt.Errorf("invalid input[%d].name: empty string", idx)
		}

		delete(itemMap, "name")
		changed = true
	}

	if !changed {
		return body, false, nil
	}

	updated, err := json.Marshal(req)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}

