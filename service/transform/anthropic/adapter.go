// Package anthropic provides a thin transform adapter that delegates to the
// upstream service/anthropic implementation.
package anthropic

import (
	"github.com/atopos31/llmio/models"
	anthropictransform "github.com/atopos31/llmio/service/anthropic"
)

// ToUnified converts an Anthropic request body into the unified request representation.
func ToUnified(rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropictransform.TransformToUnified(rawBody)
}

// FromUnified converts a unified request into an Anthropic request body.
func FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return anthropictransform.TransformFromUnified(unified)
}

// ParseResponse parses an Anthropic response body into the unified response representation.
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return anthropictransform.ParseResponse(body)
}

// FormatResponse formats a unified response as an Anthropic response body.
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return anthropictransform.FormatResponse(unified)
}
