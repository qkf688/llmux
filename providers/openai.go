package providers

import (
	"encoding/json"
	"errors"

	"github.com/atopos31/llmio/consts"
)

type OpenAI struct {
	openaiBase
}

func init() {
	Register(consts.StyleOpenAI, func(config, proxy string) (Provider, error) {
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
		Type:            consts.StyleOpenAI,
		ConfigTemplate:  configTemplateOpenAI,
		TestBody:        []byte(testBodyOpenAI),
		StructuredBody:  []byte(structuredBodyOpenAI),
		HealthCheckBody: []byte(healthCheckBodyOpenAI),
	})
}
