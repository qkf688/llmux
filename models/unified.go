package models

import (
	"encoding/json"
	"errors"
)

// UnifiedRequest 统一请求格式
// 参考 Octopus InternalLLMRequest 设计，整合所有必要的参数
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
	FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
	Seed             *int64            `json:"seed,omitempty"`
	LogitBias        map[string]int64  `json:"logit_bias,omitempty"`
	Stop             *UnifiedStop      `json:"stop,omitempty"`
	User             *string           `json:"user,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Logprobs         *bool             `json:"logprobs,omitempty"`
	TopLogprobs      *int64            `json:"top_logprobs,omitempty"`
	MaxCompletionTokens *int64         `json:"max_completion_tokens,omitempty"`
	Store            *bool             `json:"store,omitempty"`

	// 阶段 2: 响应格式和工具增强
	ResponseFormat      *UnifiedResponseFormat `json:"response_format,omitempty"`
	ToolChoice          *UnifiedToolChoice     `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool                  `json:"parallel_tool_calls,omitempty"`
	StreamOptions       *UnifiedStreamOptions  `json:"stream_options,omitempty"`

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
	PromptCacheKey     *bool   `json:"prompt_cache_key,omitempty"`
	SafetyIdentifier   *string `json:"safety_identifier,omitempty"`
	ServiceTier        *string `json:"service_tier,omitempty"`

	// 高级控制参数
	EnableThinking *bool `json:"enable_thinking,omitempty"`

	// 帮助字段 (不会发送给供应商)
	RawRequest          []byte            `json:"-"`
	ExtraBody           json.RawMessage   `json:"-"`
	Include             []string          `json:"-"`
	TransformerMetadata map[string]string `json:"-"`
	TransformOptions    TransformOptions  `json:"-"`
	Query               map[string][]string `json:"-"`
}

// Validate 验证请求参数
func (r *UnifiedRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}

	// 检查是否是嵌入请求
	isEmbeddingRequest := r.EmbeddingInput != nil
	isChatRequest := len(r.Messages) > 0

	if isEmbeddingRequest && isChatRequest {
		return errors.New("cannot specify both messages and input")
	}

	if !isEmbeddingRequest && !isChatRequest && r.System == "" {
		return errors.New("either messages, input, or system prompt is required")
	}

	// 验证嵌入请求
	if isEmbeddingRequest {
		if r.EmbeddingInput.Single == nil && len(r.EmbeddingInput.Multiple) == 0 {
			return errors.New("embedding input cannot be empty")
		}
	}

	// 验证聊天请求
	if isChatRequest && len(r.Messages) == 0 {
		return errors.New("messages are required")
	}

	return nil
}

// ClearHelpFields 清除帮助字段
func (r *UnifiedRequest) ClearHelpFields() {
	r.RawRequest = nil
	r.ExtraBody = nil
	r.Include = nil
	r.TransformerMetadata = nil
	r.Query = nil
	
	// 清除消息中的帮助字段
	for i := range r.Messages {
		r.Messages[i].ClearHelpFields()
	}
}

// IsEmbeddingRequest 判断是否为嵌入请求
func (r *UnifiedRequest) IsEmbeddingRequest() bool {
	return r.EmbeddingInput != nil
}

// IsChatRequest 判断是否为聊天请求
func (r *UnifiedRequest) IsChatRequest() bool {
	return len(r.Messages) > 0 || r.System != ""
}

// IsImageGenerationRequest 判断是否为图像生成请求
func (r *UnifiedRequest) IsImageGenerationRequest() bool {
	for _, modality := range r.Modalities {
		if modality == "image" {
			return true
		}
	}
	return false
}

// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
	Role         string                 `json:"role"`
	Content      interface{}            `json:"content,omitempty"` // 支持 string 或 []UnifiedMessageContentPart
	ToolCalls    []UnifiedToolCall      `json:"tool_calls,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	CacheControl *CacheControl          `json:"cache_control,omitempty"`
	
	// Extended Thinking 支持
	ReasoningContent *string `json:"reasoning_content,omitempty"`
	Reasoning        *string `json:"reasoning,omitempty"`
	ReasoningSignature *string `json:"reasoning_signature,omitempty"`
	
	// 帮助字段
	MessageIndex     *int    `json:"-"`
	ToolCallName     *string `json:"-"`
	ToolCallIsError  *bool   `json:"-"`
}

// ClearHelpFields 清除帮助字段
func (m *UnifiedMessage) ClearHelpFields() {
	m.ReasoningContent = nil
	m.Reasoning = nil
	m.ReasoningSignature = nil
	m.MessageIndex = nil
	m.ToolCallName = nil
	m.ToolCallIsError = nil
}

// GetReasoningContent 获取推理内容
func (m *UnifiedMessage) GetReasoningContent() string {
	if m.ReasoningContent != nil {
		return *m.ReasoningContent
	}
	if m.Reasoning != nil {
		return *m.Reasoning
	}
	return ""
}

// SetReasoningContent 设置推理内容
func (m *UnifiedMessage) SetReasoningContent(content string) {
	m.ReasoningContent = &content
}

// GetContentAsString 获取纯文本内容
func (m *UnifiedMessage) GetContentAsString() string {
	if m.Content == nil {
		return ""
	}
	if str, ok := m.Content.(string); ok {
		return str
	}
	// 如果是多模态内容，提取文本部分
	if parts, ok := m.Content.([]UnifiedMessageContentPart); ok {
		var texts []string
		for _, part := range parts {
			if part.Type == "text" && part.Text != nil {
				texts = append(texts, *part.Text)
			}
		}
		return joinStrings(texts, "")
	}
	return ""
}

// GetContentParts 获取多模态内容部分
func (m *UnifiedMessage) GetContentParts() []UnifiedMessageContentPart {
	if m.Content == nil {
		return nil
	}
	if parts, ok := m.Content.([]UnifiedMessageContentPart); ok {
		return parts
	}
	// 如果是纯文本，转换为单个 text 部分
	if str, ok := m.Content.(string); ok && str != "" {
		return []UnifiedMessageContentPart{
			{Type: "text", Text: &str},
		}
	}
	return nil
}

// SetContentString 设置纯文本内容
func (m *UnifiedMessage) SetContentString(content string) {
	m.Content = content
}

// SetContentParts 设置多模态内容
func (m *UnifiedMessage) SetContentParts(parts []UnifiedMessageContentPart) {
	m.Content = parts
}

// UnifiedToolCall 统一工具调用格式
type UnifiedToolCall struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function UnifiedToolCallFunction `json:"function"`
	Index    int                     `json:"index"`
	CacheControl *CacheControl       `json:"-"`
}

// UnifiedToolCallFunction 统一工具调用函数格式
type UnifiedToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// UnifiedTool 统一工具定义格式
type UnifiedTool struct {
	Type         string        `json:"type"`
	Function     UnifiedFunc   `json:"function"`
	CacheControl *CacheControl `json:"-"`
}

// UnifiedFunc 统一函数定义格式
type UnifiedFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

// UnifiedStop 停止序列 (支持 string 或 []string)
type UnifiedStop struct {
	Single   *string  `json:"-"`
	Multiple []string `json:"-"`
}

// MarshalJSON 自定义 JSON 序列化
func (s UnifiedStop) MarshalJSON() ([]byte, error) {
	if s.Single != nil {
		return json.Marshal(s.Single)
	}
	if len(s.Multiple) > 0 {
		return json.Marshal(s.Multiple)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化
func (s *UnifiedStop) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		s.Single = &str
		return nil
	}

	// 尝试解析为字符串数组
	var strs []string
	err = json.Unmarshal(data, &strs)
	if err == nil {
		s.Multiple = strs
		return nil
	}

	return errors.New("invalid stop type: must be string or string array")
}

// UnifiedResponseFormat 响应格式控制
type UnifiedResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// UnifiedToolChoice 工具选择控制
type UnifiedToolChoice struct {
	StringValue *string
	ObjectValue *UnifiedToolChoiceObject
}

// MarshalJSON 自定义 JSON 序列化
func (t UnifiedToolChoice) MarshalJSON() ([]byte, error) {
	if t.StringValue != nil {
		return json.Marshal(t.StringValue)
	}
	if t.ObjectValue != nil {
		return json.Marshal(t.ObjectValue)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化
func (t *UnifiedToolChoice) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		t.StringValue = &str
		return nil
	}

	// 尝试解析为对象
	var obj UnifiedToolChoiceObject
	err = json.Unmarshal(data, &obj)
	if err == nil {
		t.ObjectValue = &obj
		return nil
	}

	return errors.New("invalid tool_choice type: must be string or object")
}

// UnifiedToolChoiceObject 工具选择对象
type UnifiedToolChoiceObject struct {
	Type     string                     `json:"type"`
	Function *UnifiedToolChoiceFunction `json:"function,omitempty"`
}

// UnifiedToolChoiceFunction 工具选择函数
type UnifiedToolChoiceFunction struct {
	Name string `json:"name"`
}

// UnifiedStreamOptions 流式选项
type UnifiedStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// UnifiedAudio 音频配置
type UnifiedAudio struct {
	Voice  string `json:"voice,omitempty"`
	Format string `json:"format,omitempty"`
}

// CacheControl 缓存控制 (Anthropic 特有)
type CacheControl struct {
	Type string `json:"-"`
	TTL  string `json:"-"`
}

// UnifiedMessageContentPart 消息内容部分 (支持多种类型)
type UnifiedMessageContentPart struct {
	Type         string            `json:"type"`
	Text         *string           `json:"text,omitempty"`
	ImageURL     *UnifiedImageURL  `json:"image_url,omitempty"`
	InputAudio   *UnifiedInputAudio `json:"input_audio,omitempty"`
	CacheControl *CacheControl     `json:"-"`
}

// UnifiedImageURL 图像 URL 配置
type UnifiedImageURL struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}

// UnifiedInputAudio 音频输入配置
type UnifiedInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// UnifiedChoice 统一响应选择格式
type UnifiedChoice struct {
	Index        int             `json:"index"`
	Message      *UnifiedMessage `json:"message,omitempty"`
	Delta        *UnifiedMessage `json:"delta,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
	Logprobs     *LogprobsContent `json:"logprobs,omitempty"`
}

// UnifiedResponse 统一响应格式
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

// LogprobsContent 表示 logprobs 信息
type LogprobsContent struct {
	Content []TokenLogprob `json:"content"`
}

// TokenLogprob 表示 token 的 logprob
type TokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogprobs []TopLogprob `json:"top_logprobs,omitempty"`
}

// TopLogprob 表示顶级替代 tokens
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// ResponseError 响应错误
type ResponseError struct {
	StatusCode int         `json:"-"`
	Detail     ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// TransformOptions 转换选项 (参考 Octopus)
type TransformOptions struct {
	ArrayInputs *bool `json:"-"`
}

// UnifiedEmbeddingInput 嵌入输入 (参考 Octopus)
type UnifiedEmbeddingInput struct {
	Single   *string
	Multiple []string
}

// MarshalJSON 自定义 JSON 序列化
func (i UnifiedEmbeddingInput) MarshalJSON() ([]byte, error) {
	if i.Single != nil {
		return json.Marshal(i.Single)
	}
	if len(i.Multiple) > 0 {
		return json.Marshal(i.Multiple)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化
func (i *UnifiedEmbeddingInput) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		i.Single = &str
		return nil
	}

	var strs []string
	err = json.Unmarshal(data, &strs)
	if err == nil {
		i.Multiple = strs
		return nil
	}

	return errors.New("invalid embedding input type")
}

// 辅助函数
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// GetCompletionTokens 获取完成令牌数指针 (使用 models 包中的 Usage)
func GetCompletionTokens(u *Usage) *int64 {
	if u == nil {
		return nil
	}
	return &u.CompletionTokens
}

// GetPromptTokens 获取提示令牌数指针 (使用 models 包中的 Usage)
func GetPromptTokens(u *Usage) *int64 {
	if u == nil {
		return nil
	}
	return &u.PromptTokens
}