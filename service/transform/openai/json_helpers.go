package openai

import (
	"bytes"
	"encoding/json"
)

func decodeOpenAIChatObject(data json.RawMessage, target interface{}) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	return json.Unmarshal(data, target) == nil
}

func rawOpenAIString(data json.RawMessage) (string, bool) {
	var value string
	if len(data) == 0 || json.Unmarshal(data, &value) != nil {
		return "", false
	}
	return value, true
}
