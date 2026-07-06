package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/responses"
)

// TransformResponsesToUnified 将 Responses API 格式转换为统一格式（兼容层，委托给 responses 子包）。
func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return responses.ToUnified(ctx, rawBody)
}

// TransformUnifiedToResponses 将统一格式转换为 Responses API 格式（兼容层，委托给 responses 子包）。
func TransformUnifiedToResponses(unified *models.UnifiedRequest) ([]byte, error) {
	return responses.FromUnified(unified)
}

func parseResponsesResponse(body []byte) (*models.UnifiedResponse, error) {
	return responses.ParseResponse(body)
}

func formatResponsesResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return responses.FormatResponse(unified)
}
