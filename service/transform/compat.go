package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/anthropic"
	"github.com/atopos31/llmio/service/transform/openai"
	"github.com/atopos31/llmio/service/transform/responses"
)

// 包外兼容函数（service/transform_facade 与测试依赖）。实现直连子包，调用链 ≤ 一层委托。

// TransformOpenAIToUnified 将 OpenAI 格式转换为统一格式。
func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return openai.ToUnified(ctx, rawBody)
}

// TransformUnifiedToOpenAI 将统一格式转换为 OpenAI 格式。
func TransformUnifiedToOpenAI(unified *models.UnifiedRequest) ([]byte, error) {
	return openai.FromUnified(unified)
}

// TransformAnthropicToUnified 将 Anthropic 格式转换为统一格式。
func TransformAnthropicToUnified(rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropic.ToUnified(rawBody)
}

// TransformUnifiedToAnthropic 将统一格式转换为 Anthropic 格式。
func TransformUnifiedToAnthropic(unified *models.UnifiedRequest) ([]byte, error) {
	return anthropic.FromUnified(unified)
}

// TransformResponsesToUnified 将 Responses API 格式转换为统一格式。
func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return responses.ToUnified(ctx, rawBody)
}

// TransformUnifiedToResponses 将统一格式转换为 Responses API 格式。
func TransformUnifiedToResponses(unified *models.UnifiedRequest) ([]byte, error) {
	return responses.FromUnified(unified)
}