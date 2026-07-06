package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
)

type openAIAdapter struct{}

func init() {
	RegisterAdapter("openai", openAIAdapter{})
}

func (openAIAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return TransformOpenAIToUnified(ctx, rawBody)
}

func (openAIAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return TransformUnifiedToOpenAI(unified)
}

func (openAIAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return parseOpenAIResponse(body)
}

func (openAIAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return formatOpenAIResponse(unified)
}
