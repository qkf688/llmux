package providers

import (
	"encoding/json"
	"errors"

	"github.com/atopos31/llmio/consts"
)

// openai responses api
type OpenAIRes struct {
	openaiBase
}

func init() {
	Register(consts.StyleOpenAIRes, func(config, proxy string) (Provider, error) {
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
		Type:            consts.StyleOpenAIRes,
		ConfigTemplate:  configTemplateOpenAIRes,
		TestBody:        []byte(testBodyOpenAIRes),
		StructuredBody:  []byte(structuredBodyOpenAIRes),
		HealthCheckBody: []byte(healthCheckBodyOpenAIRes),
	})
}
