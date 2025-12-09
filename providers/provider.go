package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/atopos31/llmio/consts"
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

// New 根据类型创建对应的 Provider 实例，并注入外层存储的代理。
// proxy 参数优先级高于 config 内的代理字段，避免双处配置导致遗漏。
func New(Type, providerConfig, proxy string) (Provider, error) {
	switch Type {
	case consts.StyleOpenAI:
		var openai OpenAI
		if err := json.Unmarshal([]byte(providerConfig), &openai); err != nil {
			return nil, errors.New("invalid openai config")
		}
		if proxy != "" {
			openai.Proxy = proxy
		}

		return &openai, nil
	case consts.StyleOpenAIRes:
		var openaiRes OpenAIRes
		if err := json.Unmarshal([]byte(providerConfig), &openaiRes); err != nil {
			return nil, errors.New("invalid openai-res config")
		}
		if proxy != "" {
			openaiRes.Proxy = proxy
		}

		return &openaiRes, nil
	case consts.StyleAnthropic:
		var anthropic Anthropic
		if err := json.Unmarshal([]byte(providerConfig), &anthropic); err != nil {
			return nil, errors.New("invalid anthropic config")
		}
		if proxy != "" {
			anthropic.Proxy = proxy
		}
		return &anthropic, nil
	default:
		return nil, errors.New("unknown provider")
	}
}
