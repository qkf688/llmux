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
