package service

import (
	"context"
	"net/http"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform"
)

type TransformerManager = transform.TransformerManager

func NewTransformerManager(clientFormat, upstreamFormat consts.WireFormat) *TransformerManager {
	return transform.NewTransformerManager(clientFormat, upstreamFormat)
}

func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return transform.TransformOpenAIToUnified(ctx, rawBody)
}

func TransformUnifiedToOpenAI(unified *models.UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToOpenAI(unified)
}

func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return transform.TransformResponsesToUnified(ctx, rawBody)
}

func TransformUnifiedToResponses(unified *models.UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToResponses(unified)
}

func TransformAnthropicToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return transform.TransformAnthropicToUnified(ctx, rawBody)
}

func TransformUnifiedToAnthropic(unified *models.UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToAnthropic(unified)
}

func TransformProviderResponse(response *http.Response, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	return transform.TransformProviderResponse(response, upstreamFormat, clientFormat, sideChannel)
}
