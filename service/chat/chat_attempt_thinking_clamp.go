package chat

import (
	"log/slog"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/anthropic"
	"github.com/qkf688/llmux/service/transform"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// passthroughEffortField 返回各协议 passthrough 路径的 reasoning effort 字段路径。
// Responses 有双路径（reasoning.effort 官方字段 + metadata.reasoning_effort 兼容回退）。
func passthroughEffortFields(style string) []string {
	switch style {
	case consts.StyleOpenAI:
		return []string{"reasoning_effort"}
	case consts.StyleAnthropic:
		return []string{"output_config.effort"}
	case consts.StyleOpenAIRes:
		// 双路径：官方 reasoning.effort + 兼容 metadata.reasoning_effort
		return []string{"reasoning.effort", "metadata.reasoning_effort"}
	default:
		return nil
	}
}

// passthroughBudgetField 返回各协议 passthrough 路径的 reasoning budget 字段路径。
func passthroughBudgetFields(style string) []string {
	switch style {
	case consts.StyleOpenAI:
		return nil // OpenAI Chat API 无 budget 字段，只有 reasoning_effort
	case consts.StyleAnthropic:
		return []string{"thinking.budget_tokens"}
	case consts.StyleOpenAIRes:
		return []string{"reasoning.max_tokens"}
	default:
		return nil
	}
}

// clampPassthroughReasoning 在 passthrough 路径（同格式 1×1）对 raw body 按 style 钳制 effort 字段。
// 不违反 N×M 禁令：同格式 1×1，只处理一种 style 的字段。
// 钳制后 effort 为空串 → 删 effort 字段（none 不支持 → 剥离 thinking）。
// budget 联动（方案 E）：effort 被钳制时，同步钳 budget 上限。
func clampPassthroughReasoning(raw []byte, style string, clamp *transform.ThinkingClampConfig) []byte {
	effortFields := passthroughEffortFields(style)
	if len(effortFields) == 0 {
		return raw
	}

	// 取第一个 effort 字段的值作为钳制输入（Responses 双路径取官方字段 reasoning.effort）
	effortVal := gjson.GetBytes(raw, effortFields[0]).String()
	if effortVal == "" {
		// 官方字段为空时，Responses 尝试兼容字段 metadata.reasoning_effort
		if style == consts.StyleOpenAIRes && len(effortFields) > 1 {
			effortVal = gjson.GetBytes(raw, effortFields[1]).String()
		}
		if effortVal == "" {
			return raw
		}
	}

	// 归一化小写：与 transform 路径的 NormalizeReasoningEffort 行为一致，
	// 避免客户端传 "HIGH" 时白名单命中失败被错误降级。
	original := strings.ToLower(effortVal)
	clamped := models.ClampReasoningEffort(original, clamp.Levels, clamp.AutoFallback, clamp.UnknownStrategy)

	if clamped == original {
		// 未钳制，但若原始值非小写则需归一化写回（与 transform 路径行为一致）
		if effortVal == original {
			return raw
		}
		// 原始值大小写不规范，写回归一化小写值
		for _, field := range effortFields {
			next, err := sjson.SetBytes(raw, field, clamped)
			if err != nil {
				slog.Error("passthrough effort normalize: sjson set failed, aborting",
					"field", field, "error", err, "style", style)
				return raw
			}
			raw = next
		}
		return raw
	}

	// 钳制发生 → 更新所有 effort 字段（Responses 双路径同步）
	if clamped == "" {
		slog.Warn("passthrough reasoning effort clamped to empty (stripped)",
			"original", original,
			"style", style,
			"reason", "none_not_in_whitelist")
		// 删 effort 字段 + 空对象清理
		for _, field := range effortFields {
			if next, err := sjson.DeleteBytes(raw, field); err == nil {
				raw = next
			}
		}
		// 清理空对象（output_config / reasoning 可能变空）
		raw = cleanupEmptyPassthroughObjects(raw, style)
		// none 剥离时也删 budget 字段
		for _, field := range passthroughBudgetFields(style) {
			if next, err := sjson.DeleteBytes(raw, field); err == nil {
				raw = next
			}
		}
		raw = cleanupEmptyPassthroughObjects(raw, style)
		return raw
	}

	slog.Warn("passthrough reasoning effort clamped",
		"original", original,
		"clamped_to", clamped,
		"style", style,
		"reason", "not_in_whitelist")

	// 更新所有 effort 字段为钳制后的值。
	// fail-fast：sjson 失败通常意味着 JSON 格式异常，继续执行 budget 联动会导致
	// effort 与 budget 不匹配。失败时中止钳制，返回原 body（最坏情况是原 effort 透传）。
	for _, field := range effortFields {
		next, err := sjson.SetBytes(raw, field, clamped)
		if err != nil {
			slog.Error("passthrough effort clamp: sjson set failed, aborting clamp",
				"field", field, "error", err, "style", style)
			return raw
		}
		raw = next
	}

	// budget 联动（方案 E）：按钳制后 effort 对应 budget 值作上限
	raw = clampPassthroughBudget(raw, style, clamped)

	return raw
}

// clampPassthroughBudget 在 passthrough 路径按钳制后 effort 对应的 budget 值钳制 raw 中 budget 字段。
// budget 超上限 → 钳到上限 + warn；低于上限不动。
func clampPassthroughBudget(raw []byte, style string, clampedEffort string) []byte {
	budgetFields := passthroughBudgetFields(style)
	if len(budgetFields) == 0 {
		return raw
	}

	budgetLimit := anthropic.ReasoningEffortToThinkingBudget(clampedEffort)
	if budgetLimit <= 0 {
		return raw
	}

	for _, field := range budgetFields {
		budgetVal := gjson.GetBytes(raw, field).Int()
		if budgetVal <= 0 {
			continue
		}
		if budgetVal > budgetLimit {
			slog.Warn("passthrough reasoning budget clamped to effort limit (方案 E)",
				"field", field,
				"original_budget", budgetVal,
				"clamped_budget", budgetLimit,
				"clamped_effort", clampedEffort,
				"reason", "effort_clamped_budget_exceeds_limit")
			next, err := sjson.SetBytes(raw, field, budgetLimit)
			if err != nil {
				slog.Error("passthrough budget clamp: sjson set failed, skipping field",
					"field", field, "error", err, "style", style)
				continue // budget 单字段失败不影响其他字段，跳过即可
			}
			raw = next
		}
	}
	return raw
}

// cleanupEmptyPassthroughObjects 删除 effort/budget 字段后可能残留的空对象。
// Anthropic: output_config / thinking 变空 → 删整个键
// Responses: reasoning 变空 → 删整个键
func cleanupEmptyPassthroughObjects(raw []byte, style string) []byte {
	var objectsToCheck []string
	switch style {
	case consts.StyleAnthropic:
		objectsToCheck = []string{"output_config", "thinking"}
	case consts.StyleOpenAIRes:
		objectsToCheck = []string{"reasoning"}
	}
	for _, obj := range objectsToCheck {
		if v := gjson.GetBytes(raw, obj); v.IsObject() && len(v.Map()) == 0 {
			if next, err := sjson.DeleteBytes(raw, obj); err == nil {
				raw = next
			}
		}
	}
	return raw
}
