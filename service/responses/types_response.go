package responses

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
}

// ResponsesUsage token 使用统计。
type ResponsesUsage struct {
	InputTokens        int64                        `json:"input_tokens"`
	InputTokenDetails  *ResponsesInputTokenDetails  `json:"input_token_details,omitempty"`
	OutputTokens       int64                        `json:"output_tokens"`
	OutputTokenDetails *ResponsesOutputTokenDetails `json:"output_token_details,omitempty"`
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
	Type           string             `json:"type"`
	SequenceNumber int                `json:"sequence_number,omitempty"`
	Response       *ResponsesResponse `json:"response,omitempty"`
	OutputIndex    int                `json:"output_index,omitempty"`
	Delta          string             `json:"delta,omitempty"`
	Item           *ResponsesItem     `json:"item,omitempty"`
	Error          *ResponsesError    `json:"error,omitempty"`
}

// ResponsesError 错误信息。
type ResponsesError struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}
