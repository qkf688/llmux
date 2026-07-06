package transform

import (
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/anthropic"
)

// TransformAnthropicToUnified 将 Anthropic 格式转换为统一格式（兼容层，委托给 anthropic 子包）。
func TransformAnthropicToUnified(rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropic.ToUnified(rawBody)
}

// TransformUnifiedToAnthropic 将统一格式转换为 Anthropic 格式（兼容层，委托给 anthropic 子包）。
func TransformUnifiedToAnthropic(unified *models.UnifiedRequest) ([]byte, error) {
	return anthropic.FromUnified(unified)
}

func parseAnthropicResponse(body []byte) (*models.UnifiedResponse, error) {
	return anthropic.ParseResponse(body)
}

func formatAnthropicResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return anthropic.FormatResponse(unified)
}
