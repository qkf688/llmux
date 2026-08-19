package transform

import (
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/anthropic"
)

// clampUnifiedReasoning 在 transform 路径对 unified.ReasoningEffort 执行就近钳制 + budget 联动（方案 E）。
// 纯逻辑函数（不读设置，配置经 clamp 参数传入），slog warn 记录钳制事件。
//
// 规则：
//   - effort 为 nil 但 budget 非 nil（budget-only）→ 走 clampUnifiedBudgetOnly，按白名单最高档钳 budget
//   - effort 钳制后为空串 → 设 nil（FromUnified 不 emit thinking）
//   - effort 被钳制（clamped != original）→ budget 联动：按钳制后 effort 对应 budget 值作上限
//   - budget 超上限 → 钳到上限 + warn；低于上限不动；effort 未钳制则 budget 不动
func clampUnifiedReasoning(unified *models.UnifiedRequest, clamp *ThinkingClampConfig, clientType, providerType string) {
	if unified.ReasoningEffort == nil {
		// budget-only 请求（如 responses 入站只给 reasoning.max_tokens）：effort 缺席时
		// 仍须让 budget 受白名单约束，否则 budget 会完全绕过钳制直达上游。
		clampUnifiedBudgetOnly(unified, clamp, clientType, providerType)
		return
	}

	original := *unified.ReasoningEffort
	clamped := models.ClampReasoningEffort(original, clamp.Levels, clamp.AutoFallback, clamp.UnknownStrategy)

	// 钳制后为空串 → 剥离 thinking（设 nil）
	if clamped == "" {
		slog.Warn("reasoning effort clamped to empty (stripped)",
			"original", original,
			"client_type", clientType,
			"provider_type", providerType,
			"reason", "none_not_in_whitelist")
		unified.ReasoningEffort = nil
		// none 剥离时也清 budget（thinking 整体剥离，budget 无意义）
		unified.ReasoningBudget = nil
		return
	}

	// 钳制发生（clamped != original）→ 更新 effort + budget 联动
	if clamped != original {
		slog.Warn("reasoning effort clamped",
			"original", original,
			"clamped_to", clamped,
			"client_type", clientType,
			"provider_type", providerType,
			"reason", "not_in_whitelist")
		unified.ReasoningEffort = &clamped

		// budget 联动（方案 E）：按钳制后 effort 对应 budget 值作上限
		clampUnifiedBudget(unified, clamped)
	}
}

// clampUnifiedBudget 按钳制后 effort 对应的 budget 值作为上限，钳制 unified.ReasoningBudget。
// budget 超上限 → 钳到上限 + warn；低于上限不动（降级不限制）。
func clampUnifiedBudget(unified *models.UnifiedRequest, clampedEffort string) {
	if unified.ReasoningBudget == nil {
		return
	}
	budgetLimit := anthropic.ReasoningEffortToThinkingBudget(clampedEffort)
	if budgetLimit <= 0 {
		return
	}
	if *unified.ReasoningBudget > budgetLimit {
		slog.Warn("reasoning budget clamped to effort limit (方案 E)",
			"original_budget", *unified.ReasoningBudget,
			"clamped_budget", budgetLimit,
			"clamped_effort", clampedEffort,
			"reason", "effort_clamped_budget_exceeds_limit")
		clamped := budgetLimit
		unified.ReasoningBudget = &clamped
	}
}

// BudgetLimitUnconstrained 表示白名单不对 budget 施加上限（原样透传）。
// 取负值以与「上限 0 = 不允许思考」区分开——0 是有语义的结果，不是"没有上限"。
const BudgetLimitUnconstrained int64 = -1

// BudgetLimitForClamp 求 budget-only 请求（客户端只给 budget、不给 effort）的 budget 上限。
// 两条钳制路径（transform 走统一模型、passthrough 走 raw body）共用此口径，避免各算一套漂移。
//
// 返回值三态：
//   - > 0：上限 token 数，超出者钳到该值
//   - 0：白名单不允许任何正向思考档位（只含 none/auto）→ 调用方剥离 thinking
//   - BudgetLimitUnconstrained：不施加上限（白名单空 + unknown_strategy=passthrough）
//
// 上限取白名单最高档，而非把 budget 反推成 effort 再走 ClampReasoningEffort：
// 反推有损（513..19999 全塌到 low，正推只有 1000），会把白名单本已允许的中等 budget 过度降级。
func BudgetLimitForClamp(clamp *ThinkingClampConfig) int64 {
	if len(clamp.Levels) == 0 {
		if clamp.UnknownStrategy == "passthrough" {
			return BudgetLimitUnconstrained
		}
		// clamp_to_default：以兜底档位的 budget 作上限，与 ClampReasoningEffort
		// 对 none/auto/未知档位兜底到 autoFallback（脏值再兜到 low）的口径一致。
		fallback := clamp.AutoFallback
		if !models.IsSixLevelEffort(fallback) {
			fallback = "low"
		}
		return anthropic.ReasoningEffortToThinkingBudget(fallback)
	}
	highest := models.HighestEffortInWhitelist(clamp.Levels)
	if highest == "" {
		return 0
	}
	return anthropic.ReasoningEffortToThinkingBudget(highest)
}

// clampUnifiedBudgetOnly 处理 effort 缺席、只有 budget 的统一请求。
// 不写回反推出的 effort：那会凭空替客户端补一个它没给的字段（污染 responses 的
// reasoning.effort 与 metadata），且反推有损。这里只钳数值 / 剥离，不改出站形状。
func clampUnifiedBudgetOnly(unified *models.UnifiedRequest, clamp *ThinkingClampConfig, clientType, providerType string) {
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget <= 0 {
		return
	}

	limit := BudgetLimitForClamp(clamp)
	if limit == BudgetLimitUnconstrained {
		return
	}

	if limit == 0 {
		slog.Warn("budget-only reasoning stripped",
			"original_budget", *unified.ReasoningBudget,
			"client_type", clientType,
			"provider_type", providerType,
			"reason", "whitelist_has_no_positive_thinking_level")
		unified.ReasoningBudget = nil
		return
	}

	if *unified.ReasoningBudget > limit {
		slog.Warn("budget-only reasoning budget clamped to whitelist limit",
			"original_budget", *unified.ReasoningBudget,
			"clamped_budget", limit,
			"client_type", clientType,
			"provider_type", providerType,
			"reason", "budget_exceeds_whitelist_max_level")
		clamped := limit
		unified.ReasoningBudget = &clamped
	}
}
