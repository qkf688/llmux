package anthropic

import (
	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/models"
)

func parseCacheControl(raw interface{}) *models.CacheControl {
	cacheControl, ok := asMap(raw)
	if !ok {
		return nil
	}

	return &models.CacheControl{
		Type: maputil.String(cacheControl, "type"),
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
