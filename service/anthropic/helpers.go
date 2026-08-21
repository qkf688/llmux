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

// anthropicErrorTypes 是 Anthropic 错误响应 error.type 的官方枚举。
// 该值是客户端用来分支的机器可读标识（message 是人类可读文案、官方明说可能变动，
// 不可 pattern-match），故只允许写出协议定义过的值。
var anthropicErrorTypes = map[string]struct{}{
	"invalid_request_error": {},
	"authentication_error":  {},
	"billing_error":         {},
	"permission_error":      {},
	"not_found_error":       {},
	"request_too_large":     {},
	"rate_limit_error":      {},
	"api_error":             {},
	"overloaded_error":      {},
}

// mapAnthropicErrorType 把统一模型里的错误 type 钳到 Anthropic 官方枚举。
//
// 统一模型的 ErrorDetail.Type 来自上游协议（OpenAI 侧可能是 rate_limit_exceeded、
// server_error 这类本协议未定义的值），原样透传会让 Anthropic 客户端拿到无法分支的
// 字符串。未知/空值兜底 api_error——它是官方枚举里语义最中性的「上游内部错误」。
//
// 刻意不按 HTTP 状态码反推 type：ResponseError.StatusCode 在生产路径从未被赋值
// （非 200 上游在 chat_attempt 就转重试了，走不到本函数），按它推等于凭空造数据。
func mapAnthropicErrorType(errType string) string {
	if _, ok := anthropicErrorTypes[errType]; ok {
		return errType
	}
	return "api_error"
}

// MinThinkingBudget 是 Anthropic 扩展思考的最小合法 budget_tokens（协议硬约束）。
// 低于此值上游直接 400，故收敛 budget 时若目标值低于它，只能整体剥离 thinking
// 而不是钳到一个非法的小值。
const MinThinkingBudget int64 = 1024

// BetaInterleavedThinking 是 Anthropic 交错思考（interleaved thinking）的 beta 特性名，
// 以 anthropic-beta 头启用；llmux 侧唯一来源是 provider 配置的 Beta 字段
// （客户端自带的该头被 providers.setAnthropicBeta 清除或覆盖，进不到上游）。
//
// 它是「budget_tokens 必须严格小于 max_tokens」这条硬约束的**官方例外**：启用后 budget
// 表示「一个 assistant 轮次内所有 thinking 块的总预算」，上限变为整个上下文窗口，
// 故允许超过 max_tokens。此时若仍按常规收敛降 budget，合法请求会被静默压浅思考
// （不报错，只是想得更少），比 400 更难发现，故收敛必须先识别本例外。
const BetaInterleavedThinking = "interleaved-thinking-2025-05-14"

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
// 6 档映射的 single source of truth，供协议转换与测试复用。
//
// 高四档沿用 Octopus 值（medium→20000, high→50000, xhigh→80000, max→128000）。
// **minimal 与 low 合并到 MinThinkingBudget(1024)**：这两档原为 512 / 1000，均低于
// Anthropic 的 budget_tokens 硬地板，出站被上游直接 400（协议明文「API rejects
// smaller values」）。地板之下不存在合法值可用来表达「想得更少」，故两档只能同取地板——
// 给 low 另编一个更大的数（如 2048）是凭空发明，无官方依据；而 Anthropic 官方恰好建议
// 简单任务从 1024 最小值起步，low≈地板是有据的。
//
// 代价（刻意接受）：minimal 与 low 对 Anthropic 上游不可区分。对 OpenAI 系上游无影响，
// 那条路 effort 字符串直传、不经本函数换算。
//
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
	case "low", "minimal":
		return MinThinkingBudget
	default:
		return 0
	}
}
