package anthropic

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func getStringArray(m map[string]interface{}, key string) []string {
	values, ok := asSlice(m[key])
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

func getStringMap(m map[string]interface{}, key string) map[string]string {
	rawMap, ok := asMap(m[key])
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

func parseCacheControl(raw interface{}) *CacheControl {
	cacheControl, ok := asMap(raw)
	if !ok {
		return nil
	}

	return &CacheControl{
		Type: getString(cacheControl, "type"),
	}
}

func asMap(value interface{}) (map[string]interface{}, bool) {
	result, ok := value.(map[string]interface{})
	return result, ok
}

func asSlice(value interface{}) ([]interface{}, bool) {
	result, ok := value.([]interface{})
	return result, ok
}

func thinkingBudgetToReasoningEffort(budgetTokens int64) string {
	switch {
	case budgetTokens >= 50000:
		return "high"
	case budgetTokens >= 20000:
		return "medium"
	case budgetTokens > 0:
		return "low"
	default:
		return ""
	}
}

func reasoningEffortToThinkingBudget(effort string) int64 {
	switch effort {
	case "high":
		return 50000
	case "medium":
		return 20000
	case "low":
		return 1000
	default:
		return 0
	}
}
