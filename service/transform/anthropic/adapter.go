// Package anthropic provides a thin transform adapter that delegates to the
// upstream service/anthropic implementation.
package anthropic

import (
	"context"

	"github.com/qkf688/llmux/models"
	anthropictransform "github.com/qkf688/llmux/service/anthropic"
)

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

// ClaimedRequestKeys 返回本协议入站解析实际认领的顶层键集合。
//
// 与 openai / responses 不同，这里只做转发：Anthropic 的入站 DTO 定义在真正做解析的
// service/anthropic 包内（未导出），认领键必须与它同源反射，放到本包就得导出 DTO 或
// 手抄一份清单，两者都会漂移成误报源。
func ClaimedRequestKeys() map[string]struct{} {
	return anthropictransform.ClaimedRequestKeys()
}
