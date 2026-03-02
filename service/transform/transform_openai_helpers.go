package transform

import (
	"context"
	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
	"strings"
)

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getNestedString 支持点路径取值，如 "response.id"
func getNestedString(m map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，取字符串值
			if v, ok := current[part].(string); ok {
				return v
			}
			return ""
		}
		// 中间部分，继续深入
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return ""
		}
	}
	return ""
}

// getNestedFloat 支持点路径取值，如 "response.usage.input_tokens"
func getNestedFloat(m map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，取数值
			if v, ok := current[part].(float64); ok {
				return v
			}
			return 0
		}
		// 中间部分，继续深入
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return 0
		}
	}
	return 0
}

// getNestedMap 支持点路径取值，返回嵌套的 map
func getNestedMap(m map[string]interface{}, path string) map[string]interface{} {
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

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// 阶段 1: 新增辅助函数，支持新字段类型

// getInt 安全获取 int 值
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// getInt64 安全获取 int64 值
func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// getFloat64Ptr 安全获取 *float64 值
func getFloat64Ptr(m map[string]interface{}, key string) *float64 {
	if v, ok := m[key].(float64); ok {
		return &v
	}
	return nil
}

// getStringPtr 安全获取 *string 值
func getStringPtr(m map[string]interface{}, key string) *string {
	if v, ok := m[key].(string); ok {
		return &v
	}
	return nil
}

// getInt64Ptr 安全获取 *int64 值
func getInt64Ptr(m map[string]interface{}, key string) *int64 {
	if v, ok := m[key].(float64); ok {
		val := int64(v)
		return &val
	}
	return nil
}

// getBoolPtr 安全获取 *bool 值
func getBoolPtr(m map[string]interface{}, key string) *bool {
	if v, ok := m[key].(bool); ok {
		return &v
	}
	return nil
}

// getStringArray 安全获取 []string 值
func getStringArray(m map[string]interface{}, key string) []string {
	if arr, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// getIntMap 安全获取 map[string]int64 值
func getIntMap(m map[string]interface{}, key string) map[string]int64 {
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

// getStringMap 安全获取 map[string]string 值
func getStringMap(m map[string]interface{}, key string) map[string]string {
	if mapVal, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]string, len(mapVal))
		for k, v := range mapVal {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return nil
}

// normalizeReasoningEffort 规范化 reasoning_effort 参数
// 有效值: low, medium, high
// 映射: minimal -> low
// 无效值 -> 使用配置的默认值（默认 low）
func normalizeReasoningEffort(ctx context.Context, value string) string {
	switch strings.ToLower(value) {
	case "low", "medium", "high":
		return strings.ToLower(value)
	case "minimal":
		return "low"
	default:
		// 获取配置的默认值
		defaultValue := getReasoningEffortDefaultValue(ctx)
		if defaultValue != "" {
			return defaultValue
		}
		return "low"
	}
}

// getReasoningEffortDefaultValue 获取默认值配置
func getReasoningEffortDefaultValue(ctx context.Context) string {
	if models.DB == nil {
		return "low"
	}
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyReasoningEffortDefaultValue).
		First(ctx)
	if err != nil {
		return "low"
	}
	return setting.Value
}

// getReasoningEffortMappingEnabled 获取映射开关
func getReasoningEffortMappingEnabled(ctx context.Context) bool {
	if models.DB == nil {
		return true // 默认启用
	}
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyReasoningEffortMappingEnabled).
		First(ctx)
	if err != nil {
		return true // 默认启用
	}
	return setting.Value == "true"
}
