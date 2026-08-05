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

// ThinkingBudgetToReasoningEffort 将 thinking budget 转换为 reasoning effort。
// 参考 Octopus 实现的映射规则（single source of truth，供协议转换与测试复用）。
// 6 档反向区间——保留现有阈值不变（>=50000→high, >=20000→medium, >0→low），
// 新增 1-512→minimal, 50001-80000→xhigh, 80001+→max。
func ThinkingBudgetToReasoningEffort(budgetTokens int64) string {
	switch {
	case budgetTokens >= 80001:
		return "max"
	case budgetTokens >= 50001:
		return "xhigh"
	case budgetTokens >= 50000:
		return "high"
	case budgetTokens >= 20000:
		return "medium"
	case budgetTokens >= 1 && budgetTokens <= 512:
		return "minimal"
	case budgetTokens > 0:
		return "low"
	default:
		return ""
	}
}

// ReasoningEffortToThinkingBudget 将 reasoning effort 转换为 thinking budget。
// 参考 Octopus 实现的映射规则（single source of truth，供协议转换与测试复用）。
// 保留现有 Octopus 值不变（low→1000, medium→20000, high→50000），
// 新增 minimal→512, xhigh→80000, max→128000。
// 注意：max→128000 是标准映射值，非模型实际上限（Claude 4.6 各模型 budget max 不同，
// 如 opus 4.6=64000, sonnet=32768）。模型级精度需后续加 budget max 字段。
func ReasoningEffortToThinkingBudget(effort string) int64 {
	switch effort {
	case "max":
		return 128000
	case "xhigh":
		return 80000
	case "high":
		return 50000
	case "medium":
		return 20000
	case "low":
		return 1000
	case "minimal":
		return 512
	default:
		return 0
	}
}
