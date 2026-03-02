package service

import (
	"context"

	"github.com/atopos31/llmio/service/responses"
)

// TransformResponsesToUnified 将 Responses API 格式转换为统一格式（兼容层）。
func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error) {
	return responses.TransformRequest(rawBody, responses.RequestTransformOptions{
		MapReasoningEffort: func(effort string) string {
			if getReasoningEffortMappingEnabled(ctx) {
				return normalizeReasoningEffort(ctx, effort)
			}
			return effort
		},
	})
}

// TransformUnifiedToResponses 将统一格式转换为 Responses API 格式（兼容层）。
func TransformUnifiedToResponses(unified *UnifiedRequest) ([]byte, error) {
	return responses.TransformFromUnified(unified)
}

// parseResponsesResponse 将 Responses API 响应转换为统一格式（兼容层）。
func parseResponsesResponse(body []byte) (*UnifiedResponse, error) {
	return responses.ParseResponse(body)
}

// formatResponsesResponse 将统一格式转换为 Responses API 响应（兼容层）。
func formatResponsesResponse(unified *UnifiedResponse) ([]byte, error) {
	return responses.FormatResponse(unified)
}
