package providers

import (
	"encoding/json"
	"errors"

	"github.com/qkf688/llmux/consts"
)

// TypeOpenAI 是本 provider 在 DB Provider.Type 里的取值，也是 Register / RegisterMetadata 的键。
// 常量声明在各 provider 自己的文件里（而非集中一处），这样新增一家上游只加文件、不改公共清单（OCP）。
const TypeOpenAI = "openai"

type OpenAI struct {
	openaiBase
}

func init() {
	Register(TypeOpenAI, func(config, proxy string) (Provider, error) {
		var openai OpenAI
		if err := json.Unmarshal([]byte(config), &openai); err != nil {
			return nil, errors.New("invalid openai config")
		}
		if proxy != "" {
			openai.Proxy = proxy
		}
		openai.endpointPath = "chat/completions"
		return &openai, nil
	})
	RegisterMetadata(Metadata{
		Type:            TypeOpenAI,
		WireFormat:      consts.FormatOpenAIChat,
		ConfigTemplate:  configTemplateOpenAI,
		TestBody:        []byte(testBodyOpenAI),
		StructuredBody:  []byte(structuredBodyOpenAI),
		HealthCheckBody: []byte(healthCheckBodyOpenAI),
	})
}
