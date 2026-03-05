package responses

import (
	"encoding/json"
	"fmt"
)

// ResponsesRequest 表示 OpenAI Responses API 的请求格式。
type ResponsesRequest struct {
	Model           string                 `json:"model"`
	Instructions    string                 `json:"instructions,omitempty"`
	Input           ResponsesInput         `json:"input"`
	Text            *ResponsesText         `json:"text,omitempty"`
	Tools           []ResponsesTool        `json:"tools,omitempty"`
	ToolChoice      *ResponsesToolChoice   `json:"tool_choice,omitempty"`
	Stream          *bool                  `json:"stream,omitempty"`
	MaxOutputTokens *int                   `json:"max_output_tokens,omitempty"`
	Temperature     *float64               `json:"temperature,omitempty"`
	TopP            *float64               `json:"top_p,omitempty"`
	Reasoning       *ResponsesReasoning    `json:"reasoning,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ResponsesText 文本输出配置（用于结构化输出等）。
type ResponsesText struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

// ResponsesInput 支持 string 或 array 两种格式。
type ResponsesInput struct {
	Text  *string
	Items []ResponsesItem
}

// MarshalJSON 实现自定义 JSON 序列化。
func (i ResponsesInput) MarshalJSON() ([]byte, error) {
	if i.Text != nil {
		return json.Marshal(i.Text)
	}
	if i.Items != nil {
		items := make([]ResponsesItem, len(i.Items))
		for idx, item := range i.Items {
			items[idx] = item
			if item.Type == "output_text" && item.Annotations == nil {
				items[idx].Annotations = []ResponsesAnnotation{}
			}
		}
		return json.Marshal(items)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON 实现自定义 JSON 反序列化。
func (i *ResponsesInput) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		i.Text = nil
		i.Items = nil
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		i.Text = &text
		i.Items = nil
		return nil
	}

	var items []ResponsesItem
	if err := json.Unmarshal(data, &items); err == nil {
		i.Text = nil
		i.Items = items
		return nil
	}

	return fmt.Errorf("input must be string or array")
}

// ResponsesItem 表示输入/输出中的单个项。
type ResponsesItem struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// input_text / output_text
	Text        *string               `json:"text,omitempty"`
	Annotations []ResponsesAnnotation `json:"annotations,omitempty"`

	// function_call
	Name      *string               `json:"name,omitempty"`
	Arguments *string               `json:"arguments,omitempty"`
	CallID    *string               `json:"call_id,omitempty"`
	Status    *string               `json:"status,omitempty"`
	Output    *string               `json:"output,omitempty"`
	Options   *ResponsesTextOptions `json:"options,omitempty"`

	// Cherry Studio / octopus compatibility: role-based format
	Role    string      `json:"role,omitempty"`
	Content interface{} `json:"content,omitempty"`

	// reasoning 字段
	Summary []ResponsesReasoningSummary `json:"summary,omitempty"`
}

// ResponsesReasoningSummary reasoning 摘要。
type ResponsesReasoningSummary struct {
	Type string `json:"type"` // "summary_text"
	Text string `json:"text"`
}

// MarshalJSON 自定义序列化，确保 output_text 类型总是包含 annotations 字段。
func (item ResponsesItem) MarshalJSON() ([]byte, error) {
	type alias ResponsesItem

	if item.Type == "output_text" {
		annotations := item.Annotations
		if annotations == nil {
			annotations = []ResponsesAnnotation{}
		}

		tmp := struct {
			alias
			Annotations []ResponsesAnnotation `json:"annotations"`
		}{
			alias:       (alias)(item),
			Annotations: annotations,
		}
		return json.Marshal(tmp)
	}

	return json.Marshal((alias)(item))
}

// ResponsesAnnotation 注释（用于 output_text）。
type ResponsesAnnotation struct {
	Type string `json:"type"`
}

// ResponsesTextOptions 文本选项（用于 output_text）。
type ResponsesTextOptions struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

// ResponsesTextFormat 文本格式。
type ResponsesTextFormat struct {
	Type       string          `json:"type"`                  // "text", "json_object", or "json_schema"
	JSONSchema json.RawMessage `json:"json_schema,omitempty"` // only for "json_schema"
}

// ResponsesTool 工具定义。
// Responses API 中 name/description/parameters 直接在顶层，而非嵌套在 function 中。
type ResponsesTool struct {
	Type        string                 `json:"type"` // "function"
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ResponsesToolFunction 函数工具定义。
type ResponsesToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ResponsesToolChoice 工具选择策略。
type ResponsesToolChoice struct {
	StringValue *string
	ObjectValue *ResponsesToolChoiceObject
}

// ResponsesToolChoiceObject 对象格式的工具选择。
type ResponsesToolChoiceObject struct {
	Type     string                       `json:"type"` // "function"
	Function *ResponsesToolChoiceFunction `json:"function,omitempty"`
}

// ResponsesToolChoiceFunction 指定具体函数。
type ResponsesToolChoiceFunction struct {
	Name string `json:"name"`
}

// MarshalJSON 实现自定义 JSON 序列化。
func (tc ResponsesToolChoice) MarshalJSON() ([]byte, error) {
	if tc.StringValue != nil {
		return json.Marshal(tc.StringValue)
	}
	if tc.ObjectValue != nil {
		return json.Marshal(tc.ObjectValue)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON 实现自定义 JSON 反序列化。
func (tc *ResponsesToolChoice) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		tc.StringValue = &str
		tc.ObjectValue = nil
		return nil
	}

	var obj ResponsesToolChoiceObject
	if err := json.Unmarshal(data, &obj); err == nil {
		tc.StringValue = nil
		tc.ObjectValue = &obj
		return nil
	}

	return fmt.Errorf("tool_choice must be string or object")
}

// ResponsesReasoning 推理配置。
type ResponsesReasoning struct {
	Effort *string `json:"effort,omitempty"` // "low", "medium", "high"
}
