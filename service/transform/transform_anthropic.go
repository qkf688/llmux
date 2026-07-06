package transform

import (
	"github.com/atopos31/llmio/models"
	anthropictransform "github.com/atopos31/llmio/service/anthropic"
)

// TransformAnthropicToUnified 将 Anthropic 格式转换为统一格式（兼容层）。
func TransformAnthropicToUnified(rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropictransform.TransformToUnified(rawBody)
}

// TransformUnifiedToAnthropic 将统一格式转换为 Anthropic 格式（兼容层）。
func TransformUnifiedToAnthropic(unified *models.UnifiedRequest) ([]byte, error) {
	return anthropictransform.TransformFromUnified(unified)
}

func parseAnthropicResponse(body []byte) (*models.UnifiedResponse, error) {
	return anthropictransform.ParseResponse(body)
}

func formatAnthropicResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return anthropictransform.FormatResponse(unified)
}
