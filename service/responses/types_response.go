package responses

import "encoding/json"

// ResponsesResponse 表示 OpenAI Responses API 的响应格式。
type ResponsesResponse struct {
	Object    string                 `json:"object"` // "response"
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	CreatedAt int64                  `json:"created_at"`
	Output    []ResponsesItem        `json:"output"`
	Status    *string                `json:"status,omitempty"` // "completed", "incomplete", "failed"
	Usage     *ResponsesUsage        `json:"usage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     *ResponsesError        `json:"error,omitempty"`
}

// ResponsesUsage token 使用统计。
//
// details 的 key 必须是复数 input_tokens_details / output_tokens_details——
// 这是 OpenAI Responses API 的线格式，也是 streaming 侧编码与 chat processer 解码
// （service/chat/process.go 的 OpenAIResUsage）一直在用的写法。此处曾误用单数
// input_token_details，导致解码恒 nil（读侧成死代码）、编码输出不合规。
type ResponsesUsage struct {
	InputTokens        int64                        `json:"input_tokens"`
	InputTokenDetails  *ResponsesInputTokenDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens       int64                        `json:"output_tokens"`
	OutputTokenDetails *ResponsesOutputTokenDetails `json:"output_tokens_details,omitempty"`
	TotalTokens        int64                        `json:"total_tokens"`
}

// ResponsesInputTokenDetails 输入 token 详情。
type ResponsesInputTokenDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

// ResponsesOutputTokenDetails 输出 token 详情。
type ResponsesOutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

// ResponsesStreamEvent 表示 SSE 流式事件。
type ResponsesStreamEvent struct {
	Type           string                `json:"type"`
	SequenceNumber int                   `json:"sequence_number,omitempty"`
	ResponseID     string                `json:"response_id,omitempty"`
	Response       *ResponsesResponse    `json:"response,omitempty"`
	OutputIndex    int                   `json:"output_index,omitempty"`
	ContentIndex   int                   `json:"content_index,omitempty"`
	ItemID         string                `json:"item_id,omitempty"`
	Delta          string                `json:"delta,omitempty"`
	Text           string                `json:"text,omitempty"`
	Part           *ResponsesContentPart `json:"part,omitempty"`
	Item           *ResponsesItem        `json:"item,omitempty"`
	Error          *ResponsesError       `json:"error,omitempty"`
}

type ResponsesContentPart struct {
	Type        string                `json:"type"`
	Text        *string               `json:"text,omitempty"`
	Annotations []ResponsesAnnotation `json:"annotations,omitempty"`
	ImageURL    *json.RawMessage      `json:"image_url,omitempty"`
	URL         *string               `json:"url,omitempty"`
	Detail      *string               `json:"detail,omitempty"`
}

// ResponsesError 错误信息。
type ResponsesError struct {
	Type      string `json:"type"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
