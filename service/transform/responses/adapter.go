// Package responses provides a thin transform adapter that delegates to the
// upstream service/responses implementation.
package responses

import (
	"context"

	"github.com/atopos31/llmio/models"
	upstream "github.com/atopos31/llmio/service/responses"
	"github.com/atopos31/llmio/service/transform/shared"
)

// ToUnified converts a Responses API request body into the unified request representation.
func ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return upstream.TransformRequest(rawBody, upstream.RequestTransformOptions{
		MapReasoningEffort: func(effort string) string {
			if shared.GetReasoningEffortMappingEnabled(ctx) {
				return shared.NormalizeReasoningEffort(ctx, effort)
			}
			return effort
		},
	})
}

// FromUnified converts a unified request into a Responses API request body.
func FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return upstream.TransformFromUnified(unified)
}

// ParseResponse parses a Responses API response body into the unified response representation.
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return upstream.ParseResponse(body)
}

// FormatResponse formats a unified response as a Responses API response body.
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return upstream.FormatResponse(unified)
}
