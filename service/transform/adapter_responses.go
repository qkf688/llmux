package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/responses"
)

type responsesAdapter struct{}

func init() {
	RegisterAdapter("openai-res", responsesAdapter{})
}

func (responsesAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return responses.ToUnified(ctx, rawBody)
}

func (responsesAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return responses.FromUnified(unified)
}

func (responsesAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return responses.ParseResponse(body)
}

func (responsesAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return responses.FormatResponse(unified)
}
