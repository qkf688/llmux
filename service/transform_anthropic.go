package service

import anthropictransform "github.com/atopos31/llmio/service/anthropic"

// TransformAnthropicToUnified 将 Anthropic 格式转换为统一格式（兼容层）。
func TransformAnthropicToUnified(rawBody []byte) (*UnifiedRequest, error) {
	return anthropictransform.TransformToUnified(rawBody)
}

// TransformUnifiedToAnthropic 将统一格式转换为 Anthropic 格式（兼容层）。
func TransformUnifiedToAnthropic(unified *UnifiedRequest) ([]byte, error) {
	return anthropictransform.TransformFromUnified(unified)
}

func parseAnthropicResponse(body []byte) (*UnifiedResponse, error) {
	return anthropictransform.ParseResponse(body)
}

func formatAnthropicResponse(unified *UnifiedResponse) ([]byte, error) {
	return anthropictransform.FormatResponse(unified)
}
