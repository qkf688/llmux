package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/openai"
)

// TransformOpenAIToUnified 将 OpenAI 格式转换为统一格式（兼容层，委托给 openai 子包）。
func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return openai.ToUnified(ctx, rawBody)
}

// TransformUnifiedToOpenAI 将统一格式转换为 OpenAI 格式（兼容层，委托给 openai 子包）。
func TransformUnifiedToOpenAI(unified *models.UnifiedRequest) ([]byte, error) {
	return openai.FromUnified(unified)
}

func parseOpenAIResponse(body []byte) (*models.UnifiedResponse, error) {
	return openai.ParseResponse(body)
}

func formatOpenAIResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return openai.FormatResponse(unified)
}
