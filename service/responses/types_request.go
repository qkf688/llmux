package responses

import (
	"encoding/json"
	"fmt"
)

// ResponsesRequest 表示 OpenAI Responses API 的请求格式。
type ResponsesRequest struct {
	Model string `json:"model"`
	// Instructions 官方是**纯字符串**（openai-go 的 param.Opt[string]、openai-python 的
	// Optional[str] 都是 string），但部分客户端会写成内容段数组。这里用自定义
	// UnmarshalJSON 宽容收两种形态：数组汇总其中的文本，字符串原样。
	// 原先声明成 string 时数组形态在 json.Unmarshal 阶段就失败，整条请求 400——
	// 连 model / input 都读不到，客户端只会看到「模型是空的」这类离真因很远的报错。
	Instructions    ResponsesInstructions  `json:"instructions,omitempty"`
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
	// Responses API 原生支持这三个参数，与 Chat Completions 同名同语义。
	ServiceTier      *string `json:"service_tier,omitempty"`
	SafetyIdentifier *string `json:"safety_identifier,omitempty"`
	PromptCacheKey   *string `json:"prompt_cache_key,omitempty"`
}

// ResponsesInstructions 是 instructions 字段的多态容器：字符串或内容段数组。
//
// 只保留文本内容——instructions 的语义是「插入模型上下文的系统消息」，多模态段
// （input_image 之类）无法表达成字符串，退化成其文本部分即可，不为它把整条请求判死。
type ResponsesInstructions struct {
	Text  string
	Parts []interface{}
}

// UnmarshalJSON 实现自定义 JSON 反序列化，宽容收字符串与数组两种形态。
func (i *ResponsesInstructions) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		i.Text = str
		i.Parts = nil
		return nil
	}

	var parts []interface{}
	if err := json.Unmarshal(data, &parts); err == nil {
		i.Parts = parts
		i.Text = ""
		return nil
	}

	// 其他形状（数字 / 对象 / 布尔）：当作未设置，不让整条请求 400。
	return nil
}

// MarshalJSON 实现自定义 JSON 序列化：官方形状是字符串，数组形态原样保留。
func (i ResponsesInstructions) MarshalJSON() ([]byte, error) {
	if i.Parts != nil {
		return json.Marshal(i.Parts)
	}
	return json.Marshal(i.Text)
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
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
	CallID    *string `json:"call_id,omitempty"`
	Status    *string `json:"status,omitempty"`
	// Output 支持 string（纯文本，老上游兼容）或 []map[string]interface{}（多模态，
	// input_text/input_image 数组形式，OpenAI Responses 协议规范）。
	Output  interface{}           `json:"output,omitempty"`
	Options *ResponsesTextOptions `json:"options,omitempty"`

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
//
// **json_schema 有两种形态，都要认**：
//   - 官方扁平原 {"type":"json_schema","name":"n","schema":{...},"strict":true}
//     （openai-go SDK v2 的 responses.ResponseFormatTextJSONSchemaConfigParam 与
//     openai-python 的 ResponseFormatTextConfigParam 都是这个形状：name / schema /
//     strict 与 type 平级，无嵌套 json_schema 键）；
//   - Chat Completions 嵌套形 {"type":"json_schema","json_schema":{...}}。
//
// 入站两者归一到 unified 的内嵌对象形状（{"name","schema","strict"}，即 Chat 协议的
// 自然形状），出站按官方扁平原 emit。
type ResponsesTextFormat struct {
	Type string `json:"type"` // "text", "json_object", or "json_schema"

	// 官方扁平原的三个字段（与 type 平级）。
	Name   string                 `json:"name,omitempty"`
	Schema map[string]interface{} `json:"schema,omitempty"`
	Strict *bool                  `json:"strict,omitempty"`

	// 嵌套形（Chat Completions 风格）的 json_schema 子键。
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
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
//
// **两种形态都要认**：
//   - 官方扁平原 {"type":"function","name":"get_weather"}（OpenAI Responses 文档的形状）；
//   - Chat Completions 嵌套形 {"type":"function","function":{"name":"get_weather"}}。
//
// 只认嵌套形会让官方扁平原入站后 name 落空——「必须调用某工具」静默退化为
// 「不限制」，且日志里看不到任何丢弃痕迹。出站统一按官方扁平原 emit（见
// transform_to_responses.go），故入站两形、出站一形，回灌路径自洽。
type ResponsesToolChoiceObject struct {
	Type     string                       `json:"type"` // "function"
	Name     string                       `json:"name,omitempty"`
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
	Effort    *string `json:"effort,omitempty"`     // "low", "medium", "high"
	MaxTokens *int64  `json:"max_tokens,omitempty"` // reasoning token budget
}
