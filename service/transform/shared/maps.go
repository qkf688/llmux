// Package shared contains cross-protocol helpers used by the transform
// sub-packages (openai, responses, streaming, etc.).
//
// It intentionally does not import service/transform or any of its siblings
// to avoid circular dependencies.
package shared

import (
	"context"
	"strings"

	"github.com/atopos31/llmio/models"
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

// GetNestedMap supports dotted paths and returns the nested map.
func GetNestedMap(m map[string]interface{}, path string) map[string]interface{} {
	parts := strings.Split(path, ".")
	current := m
	for _, part := range parts {
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return nil
		}
	}
	return current
}

// GetInt safely extracts an int value from a JSON-decoded map.
func GetInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// GetFloat64Ptr safely extracts a *float64 value from a JSON-decoded map.
func GetFloat64Ptr(m map[string]interface{}, key string) *float64 {
	if v, ok := m[key].(float64); ok {
		return &v
	}
	return nil
}

// GetStringPtr safely extracts a *string value from a JSON-decoded map.
func GetStringPtr(m map[string]interface{}, key string) *string {
	if v, ok := m[key].(string); ok {
		return &v
	}
	return nil
}

// GetInt64Ptr safely extracts a *int64 value from a JSON-decoded map.
func GetInt64Ptr(m map[string]interface{}, key string) *int64 {
	if v, ok := m[key].(float64); ok {
		val := int64(v)
		return &val
	}
	return nil
}

// GetBoolPtr safely extracts a *bool value from a JSON-decoded map.
func GetBoolPtr(m map[string]interface{}, key string) *bool {
	if v, ok := m[key].(bool); ok {
		return &v
	}
	return nil
}

// GetIntMap safely extracts a map[string]int64 value from a JSON-decoded map.
func GetIntMap(m map[string]interface{}, key string) map[string]int64 {
	if mapVal, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]int64, len(mapVal))
		for k, v := range mapVal {
			if num, ok := v.(float64); ok {
				result[k] = int64(num)
			}
		}
		return result
	}
	return nil
}

// NormalizeReasoningEffort normalizes the reasoning_effort parameter.
// Valid values: low, medium, high. "minimal" maps to "low".
// Invalid values fall back to the configured default (default low).
func NormalizeReasoningEffort(ctx context.Context, value string) string {
	switch strings.ToLower(value) {
	case "low", "medium", "high":
		return strings.ToLower(value)
	case "minimal":
		return "low"
	default:
		defaultValue := getReasoningEffortDefaultValue(ctx)
		if defaultValue != "" {
			return defaultValue
		}
		return "low"
	}
}

func getReasoningEffortDefaultValue(ctx context.Context) string {
	return models.GetSettingString(ctx, models.SettingKeyReasoningEffortDefaultValue, "low")
}

// GetReasoningEffortMappingEnabled returns whether reasoning_effort mapping is enabled.
func GetReasoningEffortMappingEnabled(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyReasoningEffortMappingEnabled, true)
}
