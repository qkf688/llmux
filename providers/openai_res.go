package providers

import (
	"encoding/json"
	"errors"

	"github.com/qkf688/llmux/consts"
)

// TypeOpenAIRes 是本 provider 在 DB Provider.Type 里的取值，也是 Register / RegisterMetadata 的键。
const TypeOpenAIRes = "openai-res"

// openai responses api
type OpenAIRes struct {
	openaiBase
}

func init() {
	Register(TypeOpenAIRes, func(config, proxy string) (Provider, error) {
		var openaiRes OpenAIRes
		if err := json.Unmarshal([]byte(config), &openaiRes); err != nil {
			return nil, errors.New("invalid openai-res config")
		}
		if proxy != "" {
			openaiRes.Proxy = proxy
		}
		openaiRes.endpointPath = "responses"
		return &openaiRes, nil
	})
	RegisterMetadata(Metadata{
		Type:            TypeOpenAIRes,
		WireFormat:      consts.FormatOpenAIResponses,
		ConfigTemplate:  configTemplateOpenAIRes,
		TestBody:        []byte(testBodyOpenAIRes),
		StructuredBody:  []byte(structuredBodyOpenAIRes),
		HealthCheckBody: []byte(healthCheckBodyOpenAIRes),
	})
}
