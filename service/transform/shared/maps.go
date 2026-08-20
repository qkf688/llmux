// Package shared contains cross-protocol helpers used by the transform
// sub-packages (openai, responses, streaming, etc.).
//
// It intentionally does not import service/transform or any of its siblings
// to avoid circular dependencies.
package shared

import (
	"context"
	"strings"

	"github.com/qkf688/llmux/models"
)

// GetNestedString supports dotted paths such as "response.id".
func GetNestedString(m map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			if v, ok := current[part].(string); ok {
				return v
			}
			return ""
		}
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return ""
		}
	}
	return ""
}

// GetNestedFloat supports dotted paths such as "response.usage.input_tokens".
func GetNestedFloat(m map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			if v, ok := current[part].(float64); ok {
				return v
			}
			return 0
		}
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return 0
		}
	}
	return 0
}

// NormalizeReasoningEffort 只做字符串归一化（小写），不做模型能力钳制。
// 6 档有序 [minimal, low, medium, high, xhigh, max] + 2 特殊 [none, auto] 原样归一化返回；
// 未知档位不再回退默认，原样小写透传（钳制由 models.ClampReasoningEffort 在 chat 主路径完成）。
func NormalizeReasoningEffort(ctx context.Context, value string) string {
	return strings.ToLower(value)
}

// GetReasoningEffortMappingEnabled returns whether reasoning_effort mapping is enabled.
func GetReasoningEffortMappingEnabled(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyReasoningEffortMappingEnabled, true)
}
