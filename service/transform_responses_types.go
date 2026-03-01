package service

import (
	"encoding/json"
	"fmt"
)

// ResponsesRequest 表示 OpenAI Responses API 的请求格式
type ResponsesRequest struct {
	Model           string                 `json:"model"`
	Instructions    string                 `json:"instructions,omitempty"`
	Input           ResponsesInput         `json:"input"`
	Tools           []ResponsesTool        `json:"tools,omitempty"`
	ToolChoice      *ResponsesToolChoice   `json:"tool_choice,omitempty"`
	Stream          *bool                  `json:"stream,omitempty"`
	MaxOutputTokens *int                   `json:"max_output_tokens,omitempty"`
	Temperature     *float64               `json:"temperature,omitempty"`
	TopP            *float64               `json:"top_p,omitempty"`
	Reasoning       *ResponsesReasoning    `json:"reasoning,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ResponsesInput 支持 string 或 array 两种格式
// 简单情况：单个用户消息 → string
// 复杂情况：多轮对话 → array of ResponsesItem
type ResponsesInput struct {
	Text  *string
	Items []ResponsesItem
}

// MarshalJSON 实现自定义 JSON 序列化
// 如果 Text 非空，序列化为 string；否则序列化为 array
func (i ResponsesInput) MarshalJSON() ([]byte, error) {
	if i.Text != nil {
		return json.Marshal(i.Text)
	}
	if i.Items != nil {
		// 确保 output_text 类型的 annotations 字段存在
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

// UnmarshalJSON 实现自定义 JSON 反序列化
// 根据 JSON 类型自动识别 string 或 array
func (i *ResponsesInput) UnmarshalJSON(data []byte) error {
	// 处理 null
	if string(data) == "null" {
		i.Text = nil
		i.Items = nil
		return nil
	}

	// 尝试解析为 string
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		i.Text = &text
		i.Items = nil
		return nil
	}

	// 尝试解析为 array
	var items []ResponsesItem
	if err := json.Unmarshal(data, &items); err == nil {
		i.Text = nil
		i.Items = items
		return nil
	}

	return fmt.Errorf("input must be string or array")
}

// ResponsesItem 表示输入/输出中的单个项
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

// ResponsesReasoningSummary reasoning 摘要
type ResponsesReasoningSummary struct {
	Type string `json:"type"` // "summary_text"
	Text string `json:"text"`
}

// MarshalJSON 自定义序列化，确保 output_text 类型总是包含 annotations 字段
func (item ResponsesItem) MarshalJSON() ([]byte, error) {
	type Alias ResponsesItem

	if item.Type == "output_text" {
		annotations := item.Annotations
		if annotations == nil {
			annotations = []ResponsesAnnotation{}
		}

		// 对 output_text 强制输出 annotations 字段（不使用 omitempty）
		tmp := struct {
			Alias
			Annotations []ResponsesAnnotation `json:"annotations"`
		}{
			Alias:       (Alias)(item),
			Annotations: annotations,
		}
		return json.Marshal(tmp)
	}

	return json.Marshal((Alias)(item))
}

// ResponsesAnnotation 注释（用于 output_text）
type ResponsesAnnotation struct {
	Type string `json:"type"`
}

// ResponsesTextOptions 文本选项（用于 output_text）
type ResponsesTextOptions struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

// ResponsesTextFormat 文本格式
type ResponsesTextFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// ResponsesTool 工具定义
type ResponsesTool struct {
	Type     string                 `json:"type"` // "function"
	Function *ResponsesToolFunction `json:"function,omitempty"`
}

// ResponsesToolFunction 函数工具定义
type ResponsesToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ResponsesToolChoice 工具选择策略
// 支持 string ("auto", "none", "required") 或 object 格式
type ResponsesToolChoice struct {
	StringValue *string
	ObjectValue *ResponsesToolChoiceObject
}

// ResponsesToolChoiceObject 对象格式的工具选择
type ResponsesToolChoiceObject struct {
	Type     string                       `json:"type"` // "function"
	Function *ResponsesToolChoiceFunction `json:"function,omitempty"`
}

// ResponsesToolChoiceFunction 指定具体函数
type ResponsesToolChoiceFunction struct {
	Name string `json:"name"`
}

// MarshalJSON 实现自定义 JSON 序列化
func (tc ResponsesToolChoice) MarshalJSON() ([]byte, error) {
	if tc.StringValue != nil {
		return json.Marshal(tc.StringValue)
	}
	if tc.ObjectValue != nil {
		return json.Marshal(tc.ObjectValue)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON 实现自定义 JSON 反序列化
func (tc *ResponsesToolChoice) UnmarshalJSON(data []byte) error {
	// 尝试解析为 string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		tc.StringValue = &str
		tc.ObjectValue = nil
		return nil
	}

	// 尝试解析为 object
	var obj ResponsesToolChoiceObject
	if err := json.Unmarshal(data, &obj); err == nil {
		tc.StringValue = nil
		tc.ObjectValue = &obj
		return nil
	}

	return fmt.Errorf("tool_choice must be string or object")
}

// ResponsesReasoning 推理配置
type ResponsesReasoning struct {
	Effort *string `json:"effort,omitempty"` // "low", "medium", "high"
}

// ResponsesResponse 表示 OpenAI Responses API 的响应格式
type ResponsesResponse struct {
	Object    string                 `json:"object"` // "response"
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	CreatedAt int64                  `json:"created_at"`
	Output    []ResponsesItem        `json:"output"`
	Status    *string                `json:"status,omitempty"` // "completed", "incomplete", "failed"
	Usage     *ResponsesUsage        `json:"usage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ResponsesUsage token 使用统计
type ResponsesUsage struct {
	InputTokens        int64                        `json:"input_tokens"`
	InputTokenDetails  *ResponsesInputTokenDetails  `json:"input_token_details,omitempty"`
	OutputTokens       int64                        `json:"output_tokens"`
	OutputTokenDetails *ResponsesOutputTokenDetails `json:"output_token_details,omitempty"`
	TotalTokens        int64                        `json:"total_tokens"`
}

// ResponsesInputTokenDetails 输入 token 详情
type ResponsesInputTokenDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

// ResponsesOutputTokenDetails 输出 token 详情
type ResponsesOutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

// ResponsesStreamEvent 表示 SSE 流式事件
type ResponsesStreamEvent struct {
	Type           string             `json:"type"`
	SequenceNumber int                `json:"sequence_number,omitempty"`
	Response       *ResponsesResponse `json:"response,omitempty"`
	OutputIndex    int                `json:"output_index,omitempty"`
	Delta          string             `json:"delta,omitempty"`
	Item           *ResponsesItem     `json:"item,omitempty"`
	Error          *ResponsesError    `json:"error,omitempty"`
}

// ResponsesError 错误信息
type ResponsesError struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}
