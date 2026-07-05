package maputil

func String(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func Bool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func Float64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func Int64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func StringSlice(m map[string]interface{}, key string) []string {
	values, ok := m[key].([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if str, ok := value.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func StringMap(m map[string]interface{}, key string) map[string]string {
	rawMap, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}

	result := make(map[string]string, len(rawMap))
	for k, v := range rawMap {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}
