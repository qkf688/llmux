package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/anthropic"
)

type anthropicAdapter struct{}

func init() {
	RegisterAdapter("anthropic", anthropicAdapter{})
}

func (anthropicAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropic.ToUnified(rawBody)
}

func (anthropicAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return anthropic.FromUnified(unified)
}

func (anthropicAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return anthropic.ParseResponse(body)
}

func (anthropicAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return anthropic.FormatResponse(unified)
}
