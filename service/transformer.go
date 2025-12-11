package service

import (
	"context"
	"net/http"

	"github.com/atopos31/llmio/models"
)

// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
	Role      string            `json:"role"`
	Content   interface{}       `json:"content,omitempty"`
	ToolCalls []UnifiedToolCall `json:"tool_calls,omitempty"`
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

// UnifiedRequest 统一请求格式
type UnifiedRequest struct {
	Model       string           `json:"model"`
	Messages    []UnifiedMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	Tools       []UnifiedTool    `json:"tools,omitempty"`
	System      string           `json:"system,omitempty"`
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
		unified, err = TransformOpenAIToUnified(rawBody)
	case "anthropic":
		unified, err = TransformAnthropicToUnified(rawBody)
	default:
		unified, err = TransformOpenAIToUnified(rawBody)
	}

	if err != nil {
		return nil, err
	}

	// 2. 统一格式 -> 上游供应商格式
	switch tm.providerType {
	case "openai":
		return TransformUnifiedToOpenAI(unified)
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
