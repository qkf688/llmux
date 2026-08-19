// Package anthropic provides a thin transform adapter that delegates to the
// upstream service/anthropic implementation.
package anthropic

import (
	"context"

	"github.com/qkf688/llmux/models"
	anthropictransform "github.com/qkf688/llmux/service/anthropic"
)

// 本包刻意没有 openai / responses 那样的 ClaimedRequestKeys：Anthropic 入站没有
// 请求 DTO（service/anthropic/request_inbound.go 直接 unmarshal 到 map 再用
// maputil 逐键取值），已认领的键只以散落的字符串字面量存在，无从反射。要支持
// 未知字段检测得先给它补一个请求 DTO 或键常量表——那是独立工作量，不是漏做。

// ToUnified converts an Anthropic request body into the unified request representation.
func ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return anthropictransform.TransformToUnified(ctx, rawBody)
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
