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

func isOpenAINullRaw(data json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func rawOpenAIValue(data json.RawMessage) interface{} {
	if len(data) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
}

func rawOpenAIString(data json.RawMessage) (string, bool) {
	var value string
	if len(data) == 0 || json.Unmarshal(data, &value) != nil {
		return "", false
	}
	return value, true
}
