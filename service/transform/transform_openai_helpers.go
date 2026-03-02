package transform

import (
	"context"
	"encoding/json"
	"fmt"
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

// fieldOrder 定义各事件类型的字段顺序
var fieldOrder = map[string][]string{
	"response.created":                       {"type", "sequence_number", "response"},
	"response.in_progress":                   {"type", "sequence_number", "response"},
	"response.output_item.added":             {"type", "sequence_number", "output_index", "item"},
	"response.content_part.added":            {"type", "sequence_number", "output_index", "item_id", "content_index", "part"},
	"response.output_text.delta":             {"type", "sequence_number", "output_index", "item_id", "content_index", "delta"},
	"response.output_text.done":              {"type", "sequence_number", "output_index", "item_id", "content_index", "text"},
	"response.content_part.done":             {"type", "sequence_number", "output_index", "item_id", "content_index", "part"},
	"response.output_item.done":              {"type", "sequence_number", "output_index", "item"},
	"response.completed":                     {"type", "sequence_number", "response"},
	"response.reasoning_summary_part.added":  {"type", "sequence_number", "output_index", "item_id", "summary_index", "part"},
	"response.reasoning_summary_text.delta":  {"type", "sequence_number", "output_index", "item_id", "summary_index", "delta"},
	"response.reasoning_summary_text.done":   {"type", "sequence_number", "output_index", "item_id", "summary_index", "text"},
	"response.reasoning_summary_part.done":   {"type", "sequence_number", "output_index", "item_id", "summary_index", "part"},
	"response.function_call_arguments.delta": {"type", "sequence_number", "output_index", "item_id", "call_id", "delta"},
	"response.function_call_arguments.done":  {"type", "sequence_number", "output_index", "item_id", "call_id", "arguments"},
}

// nestedFieldOrder 定义嵌套对象的字段顺序
var nestedFieldOrder = map[string][]string{
	"response":              {"object", "id", "model", "created_at", "output", "status", "usage"},
	"item":                  {"id", "type", "role", "content", "status", "summary"},
	"part":                  {"type", "text"},
	"usage":                 {"input_tokens", "input_tokens_details", "output_tokens", "output_tokens_details", "total_tokens"},
	"input_tokens_details":  {"cached_tokens"},
	"output_tokens_details": {"reasoning_tokens"},
	"content_item":          {"type", "text"},
	"summary_item":          {"type", "text"},
}

// marshalWithTypeFirst 按照预定义顺序序列化 JSON
func marshalWithTypeFirst(data map[string]interface{}) ([]byte, error) {
	eventType, hasType := data["type"].(string)
	if !hasType {
		return json.Marshal(data)
	}

	// 获取该事件类型的字段顺序
	order, exists := fieldOrder[eventType]
	if !exists {
		// 如果没有预定义顺序，type 在前，其他随机
		return marshalWithOrder(data, []string{"type"})
	}

	return marshalWithOrder(data, order)
}

// marshalWithOrder 按指定顺序序列化对象
func marshalWithOrder(data map[string]interface{}, order []string) ([]byte, error) {
	result := "{"
	first := true

	// 按顺序输出字段
	for _, key := range order {
		value, exists := data[key]
		if !exists {
			continue
		}

		if !first {
			result += ","
		}
		first = false

		// 序列化值
		valueJSON, err := marshalValue(value, key)
		if err != nil {
			return nil, err
		}

		result += fmt.Sprintf(`"%s":%s`, key, valueJSON)
	}

	// 输出未在顺序中的字段（如果有）
	for key, value := range data {
		// 检查是否已经输出过
		found := false
		for _, orderedKey := range order {
			if key == orderedKey {
				found = true
				break
			}
		}
		if found {
			continue
		}

		if !first {
			result += ","
		}
		first = false

		valueJSON, err := marshalValue(value, key)
		if err != nil {
			return nil, err
		}

		result += fmt.Sprintf(`"%s":%s`, key, valueJSON)
	}

	result += "}"
	return []byte(result), nil
}

// marshalValue 序列化值，对嵌套对象应用字段顺序
func marshalValue(value interface{}, key string) (string, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		// 检查是否有预定义的字段顺序
		if order, exists := nestedFieldOrder[key]; exists {
			data, err := marshalWithOrder(v, order)
			return string(data), err
		}
		// 没有预定义顺序，使用标准序列化
		data, err := json.Marshal(v)
		return string(data), err
	case []interface{}:
		// 处理数组
		if len(v) == 0 {
			return "[]", nil
		}

		result := "["
		for i, item := range v {
			if i > 0 {
				result += ","
			}

			// 如果是 content 数组，应用 content_item 顺序
			if key == "content" {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if order, exists := nestedFieldOrder["content_item"]; exists {
						data, err := marshalWithOrder(itemMap, order)
						if err != nil {
							return "", err
						}
						result += string(data)
						continue
					}
				}
			}

			// 其他情况使用标准序列化
			itemJSON, err := json.Marshal(item)
			if err != nil {
				return "", err
			}
			result += string(itemJSON)
		}
		result += "]"
		return result, nil
	default:
		// 基本类型使用标准序列化
		data, err := json.Marshal(v)
		return string(data), err
	}
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
