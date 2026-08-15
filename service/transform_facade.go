package service

import (
	"context"
	"net/http"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform"
)

type Transformer = transform.Transformer
type TransformerManager = transform.TransformerManager

func NewTransformerManager(clientType, providerType string) *TransformerManager {
	return transform.NewTransformerManager(clientType, providerType)
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

func TransformProviderResponse(response *http.Response, providerType, clientType string, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	return transform.TransformProviderResponse(response, providerType, clientType, sideChannel)
}

func ValidateUnifiedRequest(req *models.UnifiedRequest) error {
	return transform.ValidateUnifiedRequest(req)
}
func RepairUnifiedRequest(req *models.UnifiedRequest) { transform.RepairUnifiedRequest(req) }
