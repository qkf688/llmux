package models

import unifiedmodel "github.com/qkf688/llmux/models/unified"

// Unified request/message/tooling 类型（兼容层）。
type UnifiedRequest = unifiedmodel.UnifiedRequest
type UnifiedMessage = unifiedmodel.UnifiedMessage
type UnifiedToolCall = unifiedmodel.UnifiedToolCall
type UnifiedToolCallFunction = unifiedmodel.UnifiedToolCallFunction
type UnifiedTool = unifiedmodel.UnifiedTool
type UnifiedFunc = unifiedmodel.UnifiedFunc
type UnifiedStop = unifiedmodel.UnifiedStop
type UnifiedResponseFormat = unifiedmodel.UnifiedResponseFormat
type UnifiedToolChoice = unifiedmodel.UnifiedToolChoice
type UnifiedToolChoiceObject = unifiedmodel.UnifiedToolChoiceObject
type UnifiedToolChoiceFunction = unifiedmodel.UnifiedToolChoiceFunction
type UnifiedStreamOptions = unifiedmodel.UnifiedStreamOptions
type UnifiedAudio = unifiedmodel.UnifiedAudio
type CacheControl = unifiedmodel.CacheControl
type UnifiedMessageContentPart = unifiedmodel.UnifiedMessageContentPart
type UnifiedImageURL = unifiedmodel.UnifiedImageURL
type UnifiedInputAudio = unifiedmodel.UnifiedInputAudio
type TransformOptions = unifiedmodel.TransformOptions
type UnifiedEmbeddingInput = unifiedmodel.UnifiedEmbeddingInput

// UnifiedChoice 统一响应选择格式。
type UnifiedChoice struct {
	Index        int              `json:"index"`
	Message      *UnifiedMessage  `json:"message,omitempty"`
	Delta        *UnifiedMessage  `json:"delta,omitempty"`
	FinishReason string           `json:"finish_reason,omitempty"`
	Logprobs     *LogprobsContent `json:"logprobs,omitempty"`
}

// UnifiedResponse 统一响应格式。
type UnifiedResponse struct {
	ID                string          `json:"id"`
	Object            string          `json:"object"`
	Created           int64           `json:"created"`
	Model             string          `json:"model"`
	Choices           []UnifiedChoice `json:"choices"`
	Usage             *Usage          `json:"usage,omitempty"`
	SystemFingerprint string          `json:"system_fingerprint,omitempty"`
	ServiceTier       string          `json:"service_tier,omitempty"`
	Error             *ResponseError  `json:"error,omitempty"`
}

// LogprobsContent 表示 logprobs 信息。
type LogprobsContent struct {
	Content []TokenLogprob `json:"content"`
}

// TokenLogprob 表示 token 的 logprob。
type TokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogprobs []TopLogprob `json:"top_logprobs,omitempty"`
}

// TopLogprob 表示顶级替代 tokens。
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// ResponseError 响应错误。
type ResponseError struct {
	StatusCode int         `json:"-"`
	Detail     ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情。
type ErrorDetail struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// GetCompletionTokens 获取完成令牌数指针 (使用 models 包中的 Usage)。
func GetCompletionTokens(u *Usage) *int64 {
	if u == nil {
		return nil
	}
	return &u.CompletionTokens
}

// GetPromptTokens 获取提示令牌数指针 (使用 models 包中的 Usage)。
func GetPromptTokens(u *Usage) *int64 {
	if u == nil {
		return nil
	}
	return &u.PromptTokens
}
