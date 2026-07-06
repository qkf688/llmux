package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
)

type anthropicAdapter struct{}

func init() {
	RegisterAdapter("anthropic", anthropicAdapter{})
}

func (anthropicAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return TransformAnthropicToUnified(rawBody)
}

func (anthropicAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return TransformUnifiedToAnthropic(unified)
}

func (anthropicAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return parseAnthropicResponse(body)
}

func (anthropicAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return formatAnthropicResponse(unified)
}
