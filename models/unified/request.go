package unified

import (
	"encoding/json"
	"errors"
)

// UnifiedRequest 统一请求格式。
// 参考 Octopus InternalLLMRequest 设计，整合所有必要的参数。
type UnifiedRequest struct {
	// 基础必填参数
	Model    string           `json:"model" validate:"required"`
	Messages []UnifiedMessage `json:"messages,omitempty"`
	Stream   bool             `json:"stream,omitempty"`

	// 基础生成参数
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`

	// 阶段 1: 基础高级参数 (参考 Octopus InternalLLMRequest)
	FrequencyPenalty    *float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64          `json:"presence_penalty,omitempty"`
	Seed                *int64            `json:"seed,omitempty"`
	LogitBias           map[string]int64  `json:"logit_bias,omitempty"`
	Stop                *UnifiedStop      `json:"stop,omitempty"`
	User                *string           `json:"user,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Logprobs            *bool             `json:"logprobs,omitempty"`
	TopLogprobs         *int64            `json:"top_logprobs,omitempty"`
	MaxCompletionTokens *int64            `json:"max_completion_tokens,omitempty"`
	Store               *bool             `json:"store,omitempty"`

	// 阶段 2: 响应格式和工具增强
	ResponseFormat    *UnifiedResponseFormat `json:"response_format,omitempty"`
	ToolChoice        *UnifiedToolChoice     `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
	StreamOptions     *UnifiedStreamOptions  `json:"stream_options,omitempty"`

	// 阶段 3: 多模态支持
	Modalities []string      `json:"modalities,omitempty"`
	Audio      *UnifiedAudio `json:"audio,omitempty"`

	// 工具支持
	Tools []UnifiedTool `json:"tools,omitempty"`

	// 系统消息 (特殊处理)
	System string `json:"system,omitempty"`

	// Extended Thinking 支持
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	ReasoningBudget *int64  `json:"reasoning_budget,omitempty"`

	// 新增: Octopus 中的重要参数 (LLMIO 缺失但需要的)
	// 嵌入相关参数
	EmbeddingInput          *UnifiedEmbeddingInput `json:"embedding_input,omitempty"`
	EmbeddingDimensions     *int64                 `json:"embedding_dimensions,omitempty"`
	EmbeddingEncodingFormat *string                `json:"embedding_encoding_format,omitempty"`

	// 缓存和安全相关
	PromptCacheKey   *bool   `json:"prompt_cache_key,omitempty"`
	SafetyIdentifier *string `json:"safety_identifier,omitempty"`
	ServiceTier      *string `json:"service_tier,omitempty"`

	// 高级控制参数
	EnableThinking *bool `json:"enable_thinking,omitempty"`

	// 帮助字段 (不会发送给供应商)
	RawRequest          []byte              `json:"-"`
	ExtraBody           json.RawMessage     `json:"-"`
	Include             []string            `json:"-"`
	TransformerMetadata map[string]string   `json:"-"`
	TransformOptions    TransformOptions    `json:"-"`
	Query               map[string][]string `json:"-"`
}

// SanitizedForProvider returns a copy of the request with helper fields cleared.
// It is intended to be used right before building outbound provider requests,
// to avoid leaking internal-only fields while keeping the original request intact.
func (r *UnifiedRequest) SanitizedForProvider() *UnifiedRequest {
	if r == nil {
		return nil
	}

	copied := *r
	if len(r.Messages) > 0 {
		copied.Messages = append([]UnifiedMessage(nil), r.Messages...)
	}
	copied.ClearHelpFields()
	return &copied
}

// Validate 验证请求参数。
func (r *UnifiedRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}

	isEmbeddingRequest := r.EmbeddingInput != nil
	isChatRequest := len(r.Messages) > 0

	if isEmbeddingRequest && isChatRequest {
		return errors.New("cannot specify both messages and input")
	}

	if !isEmbeddingRequest && !isChatRequest && r.System == "" {
		return errors.New("either messages, input, or system prompt is required")
	}

	if isEmbeddingRequest {
		if r.EmbeddingInput.Single == nil && len(r.EmbeddingInput.Multiple) == 0 {
			return errors.New("embedding input cannot be empty")
		}
	}

	if isChatRequest && len(r.Messages) == 0 {
		return errors.New("messages are required")
	}

	return nil
}

// ClearHelpFields 清除帮助字段。
func (r *UnifiedRequest) ClearHelpFields() {
	r.RawRequest = nil
	r.ExtraBody = nil
	r.Include = nil
	r.TransformerMetadata = nil
	r.Query = nil

	for i := range r.Messages {
		r.Messages[i].ClearHelpFields()
	}
}

// IsEmbeddingRequest 判断是否为嵌入请求。
func (r *UnifiedRequest) IsEmbeddingRequest() bool {
	return r.EmbeddingInput != nil
}

// IsChatRequest 判断是否为聊天请求。
func (r *UnifiedRequest) IsChatRequest() bool {
	return len(r.Messages) > 0 || r.System != ""
}

// IsImageGenerationRequest 判断是否为图像生成请求。
func (r *UnifiedRequest) IsImageGenerationRequest() bool {
	for _, modality := range r.Modalities {
		if modality == "image" {
			return true
		}
	}
	return false
}

// TransformOptions 转换选项 (参考 Octopus)。
type TransformOptions struct {
	ArrayInputs *bool `json:"-"`
}
