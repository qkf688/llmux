package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/atopos31/llmio/models"
)

// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
	Role       string            `json:"role"`
	Content    interface{}       `json:"content,omitempty"`
	ToolCalls  []UnifiedToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"` // OpenAI tool 角色消息的 tool_call_id
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
	Type     string      `json:"type"`
	Function UnifiedFunc `json:"function"`
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
