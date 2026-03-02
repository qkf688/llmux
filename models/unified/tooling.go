package unified

import (
	"encoding/json"
	"errors"
)

// UnifiedToolCall 统一工具调用格式。
type UnifiedToolCall struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function UnifiedToolCallFunction `json:"function"`
	Index    int                     `json:"index"`

	CacheControl *CacheControl `json:"-"`
}

// UnifiedToolCallFunction 统一工具调用函数格式。
type UnifiedToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// UnifiedTool 统一工具定义格式。
type UnifiedTool struct {
	Type         string        `json:"type"`
	Function     UnifiedFunc   `json:"function"`
	CacheControl *CacheControl `json:"-"`
}

// UnifiedFunc 统一函数定义格式。
type UnifiedFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

// UnifiedResponseFormat 响应格式控制。
type UnifiedResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// UnifiedToolChoice 工具选择控制。
type UnifiedToolChoice struct {
	StringValue *string
	ObjectValue *UnifiedToolChoiceObject
}

// MarshalJSON 自定义 JSON 序列化。
func (t UnifiedToolChoice) MarshalJSON() ([]byte, error) {
	if t.StringValue != nil {
		return json.Marshal(t.StringValue)
	}
	if t.ObjectValue != nil {
		return json.Marshal(t.ObjectValue)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化。
func (t *UnifiedToolChoice) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		t.StringValue = &str
		return nil
	}

	var obj UnifiedToolChoiceObject
	err = json.Unmarshal(data, &obj)
	if err == nil {
		t.ObjectValue = &obj
		return nil
	}

	return errors.New("invalid tool_choice type: must be string or object")
}

// UnifiedToolChoiceObject 工具选择对象。
type UnifiedToolChoiceObject struct {
	Type     string                     `json:"type"`
	Function *UnifiedToolChoiceFunction `json:"function,omitempty"`
}

// UnifiedToolChoiceFunction 工具选择函数。
type UnifiedToolChoiceFunction struct {
	Name string `json:"name"`
}

// UnifiedStreamOptions 流式选项。
type UnifiedStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// UnifiedAudio 音频配置。
type UnifiedAudio struct {
	Voice  string `json:"voice,omitempty"`
	Format string `json:"format,omitempty"`
}
