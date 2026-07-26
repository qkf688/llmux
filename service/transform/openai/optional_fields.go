package openai

import (
	"encoding/json"
)

type openAIString struct {
	Value string
	Set   bool
}

func (f *openAIString) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIBool struct {
	Value bool
	Set   bool
}

func (f *openAIBool) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIFloat struct {
	Value float64
	Set   bool
}

func (f *openAIFloat) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIInt struct {
	Value int
	Set   bool
}

func (f *openAIInt) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = int(value)
		f.Set = true
	}
	return nil
}

type openAIInt64 struct {
	Value int64
	Set   bool
}

func (f *openAIInt64) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = int64(value)
		f.Set = true
	}
	return nil
}

type openAIInt64Map struct {
	Value map[string]int64
	Set   bool
}

func (f *openAIInt64Map) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		if num, ok := value.(float64); ok {
			result[key] = int64(num)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIStringMap struct {
	Value map[string]string
	Set   bool
}

func (f *openAIStringMap) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		if str, ok := value.(string); ok {
			result[key] = str
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIStringSeq struct {
	Value []string
	Set   bool
}

func (f *openAIStringSeq) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, value := range raw {
		if str, ok := value.(string); ok {
			result = append(result, str)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIRawArray []json.RawMessage

func (a *openAIRawArray) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		*a = raw
	}
	return nil
}
