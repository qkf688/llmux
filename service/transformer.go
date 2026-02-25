package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/atopos31/llmio/models"
)

// 阶段 3: 多模态内容支持类型定义

// UnifiedMessageContent 消息内容 (支持纯文本或多模态)
// 参考 Octopus MessageContent 实现
type UnifiedMessageContent struct {
	Content         *string                     `json:"content,omitempty"`
	MultipleContent []UnifiedMessageContentPart `json:"multiple_content,omitempty"`
}

// MarshalJSON 自定义 JSON 序列化
func (c UnifiedMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.MultipleContent) > 0 {
		// 优化: 单个 text 类型直接序列化为字符串
		if len(c.MultipleContent) == 1 && c.MultipleContent[0].Type == "text" {
			return json.Marshal(c.MultipleContent[0].Text)
		}
		return json.Marshal(c.MultipleContent)
	}
	return json.Marshal(c.Content)
}

// UnmarshalJSON 自定义 JSON 反序列化
func (c *UnifiedMessageContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		c.Content = &str
		return nil
	}

	// 尝试解析为内容部分数组
	var parts []UnifiedMessageContentPart
	err = json.Unmarshal(data, &parts)
	if err == nil {
		c.MultipleContent = parts
		return nil
	}

	return errors.New("invalid content type: must be string or array of content parts")
}

// UnifiedMessageContentPart 消息内容部分 (支持多种类型)
type UnifiedMessageContentPart struct {
	// Type 内容类型: "text", "image_url", "input_audio"
	Type string `json:"type"`

	// Text 文本内容 (type="text" 时使用)
	Text *string `json:"text,omitempty"`

	// ImageURL 图像 URL (type="image_url" 时使用)
	ImageURL *UnifiedImageURL `json:"image_url,omitempty"`

	// InputAudio 音频输入 (type="input_audio" 时使用)
	InputAudio *UnifiedInputAudio `json:"input_audio,omitempty"`

	// CacheControl 缓存控制 (Anthropic 特有)
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// UnifiedImageURL 图像 URL 配置
type UnifiedImageURL struct {
	// URL 图像的 URL (支持 http/https 或 data URI)
	URL string `json:"url"`

	// Detail 图像细节级别: "auto", "low", "high"
	Detail *string `json:"detail,omitempty"`
}

// UnifiedInputAudio 音频输入配置
type UnifiedInputAudio struct {
	// Data Base64 编码的音频数据
	Data string `json:"data"`

	// Format 音频格式: "wav", "mp3"
	Format string `json:"format"`
}

// UnifiedAudio 音频输出配置
type UnifiedAudio struct {
	// Voice 语音类型: "alloy", "echo", "fable", "onyx", "nova", "shimmer"
	Voice string `json:"voice,omitempty"`

	// Format 音频格式: "wav", "mp3", "pcm16"
	Format string `json:"format,omitempty"`
}

// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
	Role         string         `json:"role"`
	Content      interface{}    `json:"content,omitempty"` // 支持 string 或 []UnifiedMessageContentPart
	ToolCalls    []UnifiedToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string         `json:"tool_call_id,omitempty"` // OpenAI tool 角色消息的 tool_call_id
	CacheControl *CacheControl  `json:"cache_control,omitempty"` // Anthropic 缓存控制
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
		return strings.Join(texts, "")
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
	CacheControl *CacheControl `json:"cache_control,omitempty"` // Anthropic 缓存控制
}

// UnifiedFunc 统一函数定义格式
type UnifiedFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// UnifiedStop 停止序列 (支持 string 或 []string)
// 参考 Octopus InternalLLMRequest.Stop 实现
type UnifiedStop struct {
	Single   *string
	Multiple []string
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
// 参考 Octopus ResponseFormat 实现
type UnifiedResponseFormat struct {
	// Type 指定响应格式类型
	// 可选值: "text", "json_object", "json_schema"
	Type string `json:"type"`
	// JSONSchema 用于 json_schema 类型的 schema 定义
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// UnifiedToolChoice 工具选择控制
// 参考 Octopus ToolChoice 实现
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
	// IncludeUsage 是否在流式响应中包含 usage 信息
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// UnifiedRequest 统一请求格式
type UnifiedRequest struct {
	Model           string           `json:"model"`
	Messages        []UnifiedMessage `json:"messages"`
	MaxTokens       int              `json:"max_tokens,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	TopP            *float64         `json:"top_p,omitempty"`
	Stream          bool             `json:"stream,omitempty"`
	Tools           []UnifiedTool    `json:"tools,omitempty"`
	System          string           `json:"system,omitempty"`
	ReasoningEffort *string          `json:"reasoning_effort,omitempty"` // 推理强度参数

	// 阶段 1: 基础高级参数 (参考 Octopus InternalLLMRequest)
	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// their existing frequency in the text so far.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// whether they appear in the text so far.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// This feature is in Beta. If specified, our system will make a best effort to
	// sample deterministically, such that repeated requests with the same seed and
	// parameters should return the same result.
	Seed *int64 `json:"seed,omitempty"`

	// Modify the likelihood of specified tokens appearing in the completion.
	// Accepts a JSON object that maps tokens (specified by their token ID) to an
	// associated bias value from -100 to 100.
	LogitBias map[string]int64 `json:"logit_bias,omitempty"`

	// Up to 4 sequences where the API will stop generating further tokens.
	Stop *UnifiedStop `json:"stop,omitempty"`

	// A unique identifier representing your end-user, which can help OpenAI to
	// monitor and detect abuse.
	User *string `json:"user,omitempty"`

	// Set of 16 key-value pairs that can be attached to an object.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Whether to return log probabilities of the output tokens or not.
	Logprobs *bool `json:"logprobs,omitempty"`

	// An integer between 0 and 20 specifying the number of most likely tokens to
	// return at each token position.
	TopLogprobs *int64 `json:"top_logprobs,omitempty"`

	// An upper bound for the number of tokens that can be generated for a completion.
	MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`

	// Whether or not to store the output of this chat completion request.
	Store *bool `json:"store,omitempty"`

	// 阶段 2: 响应格式和工具增强
	// An object specifying the format that the model must output.
	ResponseFormat *UnifiedResponseFormat `json:"response_format,omitempty"`

	// Controls which (if any) tool is called by the model.
	// Can be "none", "auto", "required", or an object specifying a particular tool.
	ToolChoice *UnifiedToolChoice `json:"tool_choice,omitempty"`

	// Whether to enable parallel function calling during tool use.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`

	// Options for streaming response. Only set this when you set stream: true.
	StreamOptions *UnifiedStreamOptions `json:"stream_options,omitempty"`

	// 阶段 3: 多模态支持
	// Output types that you would like the model to generate.
	// Most models are capable of generating text, which is the default: ["text"]
	// To generate audio: ["text", "audio"]
	// Any of "text", "audio", "image".
	Modalities []string `json:"modalities,omitempty"`

	// Parameters for audio output. Required when audio output is requested with modalities: ["audio"].
	Audio *UnifiedAudio `json:"audio,omitempty"`
}

// UnifiedChoice 统一响应选择格式
type UnifiedChoice struct {
	Index        int             `json:"index"`
	Message      *UnifiedMessage `json:"message,omitempty"`
	Delta        *UnifiedMessage `json:"delta,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

// UnifiedResponse 统一响应格式
type UnifiedResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []UnifiedChoice `json:"choices"`
	Usage   *models.Usage   `json:"usage,omitempty"`
}

// Transformer 格式转换器接口
type Transformer interface {
	// TransformRequest 将客户端请求转换为统一格式
	TransformRequest(rawBody []byte) (*UnifiedRequest, error)

	// TransformToProvider 将统一格式转换为上游供应商格式
	TransformToProvider(unified *UnifiedRequest, providerType string) ([]byte, error)

	// TransformResponse 将上游供应商响应转换为客户端格式
	TransformResponse(response *http.Response, clientType string) (*http.Response, error)
}

// TransformerManager 转换管理器
type TransformerManager struct {
	clientType   string // 客户端格式类型
	providerType string // 上游供应商类型
}

// NewTransformerManager 创建转换管理器
func NewTransformerManager(clientType, providerType string) *TransformerManager {
	return &TransformerManager{
		clientType:   clientType,
		providerType: providerType,
	}
}

// ProcessRequest 处理请求转换
func (tm *TransformerManager) ProcessRequest(ctx context.Context, rawBody []byte) ([]byte, error) {
	// 1. 客户端格式 -> 统一格式
	var unified *UnifiedRequest
	var err error

	switch tm.clientType {
	case "openai":
		unified, err = TransformOpenAIToUnified(ctx, rawBody)
	case "openai-res":
		unified, err = TransformResponsesToUnified(ctx, rawBody)
	case "anthropic":
		unified, err = TransformAnthropicToUnified(rawBody)
	default:
		unified, err = TransformOpenAIToUnified(ctx, rawBody)
	}

	if err != nil {
		return nil, err
	}

	// 2. 统一格式 -> 上游供应商格式
	switch tm.providerType {
	case "openai":
		return TransformUnifiedToOpenAI(unified)
	case "openai-res":
		return TransformUnifiedToResponses(unified)
	case "anthropic":
		return TransformUnifiedToAnthropic(unified)
	default:
		return TransformUnifiedToOpenAI(unified)
	}
}

// ProcessResponse 处理响应转换
func (tm *TransformerManager) ProcessResponse(response *http.Response) (*http.Response, error) {
	// 上游供应商格式 -> 统一格式 -> 客户端格式
	return TransformProviderResponse(response, tm.providerType, tm.clientType)
}
