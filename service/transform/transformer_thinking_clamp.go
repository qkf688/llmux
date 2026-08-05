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
//   - effort 为 nil → 不钳制（无 reasoning 字段）
//   - effort 钳制后为空串 → 设 nil（FromUnified 不 emit thinking）
//   - effort 被钳制（clamped != original）→ budget 联动：按钳制后 effort 对应 budget 值作上限
//   - budget 超上限 → 钳到上限 + warn；低于上限不动；effort 未钳制则 budget 不动
func clampUnifiedReasoning(unified *models.UnifiedRequest, clamp *ThinkingClampConfig, clientType, providerType string) {
	if unified.ReasoningEffort == nil {
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
