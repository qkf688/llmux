package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// SettingValueToString 将任意类型的值按 SettingType 序列化为字符串。
func SettingValueToString(value any, t SettingType) (string, error) {
	switch t {
	case SettingTypeBool:
		b, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("expected bool, got %T", value)
		}
		if b {
			return "true", nil
		}
		return "false", nil
	case SettingTypeInt:
		var i int
		switch v := value.(type) {
		case int:
			i = v
		case int64:
			i = int(v)
		case float64:
			i = int(v)
		default:
			return "", fmt.Errorf("expected int, got %T", value)
		}
		return strconv.Itoa(i), nil
	case SettingTypeString:
		s, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("expected string, got %T", value)
		}
		return s, nil
	case SettingTypeStringSlice:
		slice, ok := value.([]string)
		if !ok {
			return "", fmt.Errorf("expected []string, got %T", value)
		}
		b, err := json.Marshal(slice)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case SettingTypeJSON:
		b, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unsupported setting type: %s", t)
	}
}

// SettingValueFromString 将字符串按 SettingType 反序列化为任意类型。
func SettingValueFromString(value string, t SettingType) (any, error) {
	switch t {
	case SettingTypeBool:
		return value == "true", nil
	case SettingTypeInt:
		i, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid int value %q: %w", value, err)
		}
		return i, nil
	case SettingTypeString:
		return value, nil
	case SettingTypeStringSlice:
		var slice []string
		if value == "" {
			return slice, nil
		}
		if err := json.Unmarshal([]byte(value), &slice); err != nil {
			return nil, fmt.Errorf("invalid []string value %q: %w", value, err)
		}
		return slice, nil
	case SettingTypeJSON:
		var options RawLogOptions
		if err := json.Unmarshal([]byte(value), &options); err != nil {
			return nil, fmt.Errorf("invalid json value %q: %w", value, err)
		}
		return options, nil
	default:
		return nil, fmt.Errorf("unsupported setting type: %s", t)
	}
}
