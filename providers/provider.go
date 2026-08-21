package providers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"` // 使用 int64 存储 Unix 时间戳
	OwnedBy string `json:"owned_by"`
}

type Provider interface {
	BuildReq(ctx context.Context, header http.Header, model string, rawData []byte) (*http.Request, error)
	Models(ctx context.Context) ([]Model, error)
	GetProxy() string
}

// OpenAICompat 标记 OpenAI 兼容协议族（chat/completions 或 responses 风格配置导出）。
// *OpenAI / *OpenAIRes 实现；*Anthropic 不实现。供 React 测试等走统一能力接口，避免具体类型断言。
type OpenAICompat interface {
	Provider
	OpenAICompatBaseURL() string
	OpenAICompatAPIKey() string
}

// BetaFeatureCapable 暴露「本供应商是否启用了某个 beta 特性」的查询能力。
// 上层做协议合法性收敛时需要它：同一条硬约束在特定 beta 开启后可能整体失效
// （Anthropic interleaved thinking 下 thinking.budget_tokens 允许超过 max_tokens），
// 不看 beta 就收敛等于把合法请求静默改写。
// 目前仅 *Anthropic 实现；调用方**必须**断言本接口而非具体类型（OCP：新 type 想参与只需实现它）。
type BetaFeatureCapable interface {
	Provider
	HasBetaFeature(name string) bool
}

func buildCustomModels(custom []string) []Model {
	now := time.Now().Unix()
	models := make([]Model, 0, len(custom))
	for _, model := range custom {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		models = append(models, Model{
			ID:      trimmed,
			Object:  "custom",
			Created: now,
			OwnedBy: "custom",
		})
	}
	return models
}

// Factory 根据 JSON 配置与外部代理地址构造 Provider。
// 实现内部负责反序列化，并在外部 proxy 非空时覆盖配置中的 proxy。
type Factory func(config, proxy string) (Provider, error)

var factories = make(map[string]Factory)

// Register 注册一个 Provider 构造器。
// 重复注册同一类型会 panic，避免运行期覆盖导致行为不可预期。
func Register(providerType string, factory Factory) {
	if _, exists := factories[providerType]; exists {
		panic("provider factory already registered: " + providerType)
	}
	factories[providerType] = factory
}

// New 根据类型从注册表创建 Provider 实例。
// proxy 参数优先级高于 config 内的代理字段，避免双处配置导致遗漏。
func New(providerType, providerConfig, proxy string) (Provider, error) {
	factory, ok := factories[providerType]
	if !ok {
		return nil, errors.New("unknown provider")
	}
	return factory(providerConfig, proxy)
}
