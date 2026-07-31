package transform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/qkf688/llmux/models"
)

// 阶段 3: 多模态内容支持类型定义
// 注意：这些类型现在在 models/unified.go 中定义，这里保留是为了向后兼容

// UnifiedMessageContent 消息内容 (支持纯文本或多模态)
// 参考 Octopus MessageContent 实现
type UnifiedMessageContent struct {
	Content         *string                            `json:"content,omitempty"`
	MultipleContent []models.UnifiedMessageContentPart `json:"multiple_content,omitempty"`
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
	var parts []models.UnifiedMessageContentPart
	err = json.Unmarshal(data, &parts)
	if err == nil {
		c.MultipleContent = parts
		return nil
	}

	return errors.New("invalid content type: must be string or array of content parts")
}

// Transformer 格式转换器接口
type Transformer interface {
	// TransformRequest 将客户端请求转换为统一格式
	TransformRequest(rawBody []byte) (*models.UnifiedRequest, error)

	// TransformToProvider 将统一格式转换为上游供应商格式
	TransformToProvider(unified *models.UnifiedRequest, providerType string) ([]byte, error)

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
	clientAdapter, err := getAdapterOrDefault(tm.clientType)
	if err != nil {
		return nil, err
	}
	unified, err := clientAdapter.ToUnified(ctx, rawBody)
	if err != nil {
		return nil, err
	}

	providerAdapter, err := getAdapterOrDefault(tm.providerType)
	if err != nil {
		return nil, err
	}
	return providerAdapter.FromUnified(unified)
}

// ProcessResponse 处理响应转换
func (tm *TransformerManager) ProcessResponse(response *http.Response) (*http.Response, error) {
	// 上游供应商格式 -> 统一格式 -> 客户端格式
	return TransformProviderResponse(response, tm.providerType, tm.clientType)
}
