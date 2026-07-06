package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
)

type responsesAdapter struct{}

func init() {
	RegisterAdapter("openai-res", responsesAdapter{})
}

func (responsesAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return TransformResponsesToUnified(ctx, rawBody)
}

func (responsesAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return TransformUnifiedToResponses(unified)
}

func (responsesAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return parseResponsesResponse(body)
}

func (responsesAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return formatResponsesResponse(unified)
}
