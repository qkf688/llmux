package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/openai"
)

type openAIAdapter struct{}

func init() {
	RegisterAdapter("openai", openAIAdapter{})
}

func (openAIAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return openai.ToUnified(ctx, rawBody)
}

func (openAIAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return openai.FromUnified(unified)
}

func (openAIAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return openai.ParseResponse(body)
}

func (openAIAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return openai.FormatResponse(unified)
}
