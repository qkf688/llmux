package anthropic

import (
	"encoding/json"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

// parseCacheControl 把 cache_control 解成统一模型的缓存标记。
//
// 注意 `{"cache_control":{}}` 返回的是 &CacheControl{Type:""} 而非 nil——键存在即
// 表示客户端声明了缓存意图，type 缺失是另一回事。
func parseCacheControl(raw json.RawMessage) *models.CacheControl {
	var cacheControl anthropicCacheControl
	if !shared.DecodeJSONObject(raw, &cacheControl) {
		return nil
	}

	return &models.CacheControl{
		Type: cacheControl.Type.Value,
	}
}

// asMap / asSlice 现在只服务**响应**入站解析（response_inbound.go 的 ParseResponse
// 与 extractThinking 仍走 map[string]interface{}）。请求入站已全量走 struct DTO，
// 不要在请求侧新增这两个的调用。
func asMap(value interface{}) (map[string]interface{}, bool) {
	result, ok := value.(map[string]interface{})
	return result, ok
}

func asSlice(value interface{}) ([]interface{}, bool) {
	result, ok := value.([]interface{})
	return result, ok
}

// MinThinkingBudget 是 Anthropic 扩展思考的最小合法 budget_tokens（协议硬约束）。
// 低于此值上游直接 400，故收敛 budget 时若目标值低于它，只能整体剥离 thinking
// 而不是钳到一个非法的小值。
const MinThinkingBudget int64 = 1024

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
