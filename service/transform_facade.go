package service

import (
	"context"
	"net/http"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform"
)

type UnifiedMessageContent = transform.UnifiedMessageContent

type UnifiedMessage = models.UnifiedMessage
type UnifiedToolCall = models.UnifiedToolCall
type UnifiedToolCallFunction = models.UnifiedToolCallFunction
type UnifiedTool = models.UnifiedTool
type UnifiedFunc = models.UnifiedFunc
type UnifiedStop = models.UnifiedStop
type UnifiedResponseFormat = models.UnifiedResponseFormat
type UnifiedToolChoice = models.UnifiedToolChoice
type UnifiedToolChoiceObject = models.UnifiedToolChoiceObject
type UnifiedToolChoiceFunction = models.UnifiedToolChoiceFunction
type UnifiedStreamOptions = models.UnifiedStreamOptions
type UnifiedAudio = models.UnifiedAudio
type CacheControl = models.CacheControl
type UnifiedChoice = models.UnifiedChoice
type UnifiedResponse = models.UnifiedResponse
type UnifiedRequest = models.UnifiedRequest

type UnifiedMessageContentPart = models.UnifiedMessageContentPart
type UnifiedImageURL = models.UnifiedImageURL
type UnifiedInputAudio = models.UnifiedInputAudio

type Transformer = transform.Transformer
type TransformerManager = transform.TransformerManager

func NewTransformerManager(clientType, providerType string) *TransformerManager {
	return transform.NewTransformerManager(clientType, providerType)
}

func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error) {
	return transform.TransformOpenAIToUnified(ctx, rawBody)
}

func TransformUnifiedToOpenAI(unified *UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToOpenAI(unified)
}

func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error) {
	return transform.TransformResponsesToUnified(ctx, rawBody)
}

func TransformUnifiedToResponses(unified *UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToResponses(unified)
}

func TransformAnthropicToUnified(rawBody []byte) (*UnifiedRequest, error) {
	return transform.TransformAnthropicToUnified(rawBody)
}

func TransformUnifiedToAnthropic(unified *UnifiedRequest) ([]byte, error) {
	return transform.TransformUnifiedToAnthropic(unified)
}

func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
	return transform.TransformProviderResponse(response, providerType, clientType)
}

func ValidateUnifiedRequest(req *UnifiedRequest) error { return transform.ValidateUnifiedRequest(req) }
func RepairUnifiedRequest(req *UnifiedRequest)         { transform.RepairUnifiedRequest(req) }
