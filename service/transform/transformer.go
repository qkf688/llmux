package transform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

// ThinkingClampConfig 携带模型思考档位白名单 + 策略设置进入 ProcessRequest。
// 所有字段由调用方预先从 ctx + model/association 解析后传入，ProcessRequest 保持纯函数（不读设置）。
// Clamp 为 nil 表示不做钳制（如 SupportsThinking=false 时 thinking 已被 stripThinkingFields 剥离）。
type ThinkingClampConfig struct {
	Levels          []string // ThinkingLevelsResolved 结果；nil/空=白名单空
	AutoFallback    string   // SettingKeyReasoningEffortDefaultValue（auto 不支持且白名单空时兜底）
	UnknownStrategy string   // SettingKeyReasoningEffortUnknownStrategy（clamp_to_default / passthrough）
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

// ProcessRequest 处理请求转换。
// clamp 为 nil 时跳过思考档位钳制（thinking 已被上游剥离或模型不支持）。
// 钳制在 ToUnified 后、FromUnified 前对 unified.ReasoningEffort 执行（走 unified 合规 4.4 节）。
// 钳制后 effort 为空串 → 设 nil（FromUnified 自然不 emit thinking 字段）。
// budget 联动（方案 E）：effort 被钳制时，按钳制后 effort 对应 budget 值作上限，
// 超上限则钳到上限 + warn；低于上限不动；effort 未钳制则 budget 不动。
func (tm *TransformerManager) ProcessRequest(ctx context.Context, rawBody []byte, clamp *ThinkingClampConfig) ([]byte, error) {
	clientAdapter, err := getAdapterOrDefault(tm.clientType)
	if err != nil {
		return nil, err
	}
	unified, err := clientAdapter.ToUnified(ctx, rawBody)
	if err != nil {
		return nil, err
	}

	// 思考档位钳制（transform 路径，走 unified）
	if clamp != nil {
		clampUnifiedReasoning(unified, clamp, tm.clientType, tm.providerType)
	}

	providerAdapter, err := getAdapterOrDefault(tm.providerType)
	if err != nil {
		return nil, err
	}
	return providerAdapter.FromUnified(unified)
}

// ProcessResponse 处理响应转换
func (tm *TransformerManager) ProcessResponse(response *http.Response, rawAccumulator *strings.Builder) (*http.Response, error) {
	// 上游供应商格式 -> 统一格式 -> 客户端格式
	return TransformProviderResponse(response, tm.providerType, tm.clientType, rawAccumulator)
}
