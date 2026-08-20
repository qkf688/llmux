package chat

import (
	"log/slog"
	"math"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/anthropic"
	"github.com/qkf688/llmux/service/transform"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// budgetMaxTokensRatio 是收敛非法 budget 时目标 budget 占 max_tokens 的比例。
// Anthropic 的 max_tokens 是「思考 + 回答」总额；留 20% 给回答，避免思考吃满导致回答无空间。
// 这是 chat 层的收敛策略（非协议事实），故常量定义在本包而非 service/anthropic。
const budgetMaxTokensRatio = 0.8

// passthroughEffortFields 返回各协议 passthrough 路径的 reasoning effort 字段路径。
func passthroughEffortFields(style string) []string {
	switch style {
	case consts.StyleOpenAI:
		return []string{"reasoning_effort"}
	case consts.StyleAnthropic:
		return []string{"output_config.effort"}
	case consts.StyleOpenAIRes:
		return []string{"reasoning.effort"}
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

// passthroughThinkingContainers 返回剥离 thinking 时必须整体删除的容器键。
// Anthropic 的 thinking 只有 type + budget_tokens 两个键，删掉 budget 后残留的
// {"type":"enabled"} 缺 budget_tokens，上游会 400——必须连容器一起删。
// Responses 的 reasoning 还可能带 summary 等无关键，只能靠空对象清理，不在此列。
func passthroughThinkingContainers(style string) []string {
	switch style {
	case consts.StyleAnthropic:
		return []string{"thinking"}
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

	// 取 effort 字段的值作为钳制输入
	effortVal := gjson.GetBytes(raw, effortFields[0]).String()
	if effortVal == "" {
		// budget-only 请求（只给 budget、不给 effort）：effort 缺席时仍须让 budget
		// 受白名单约束，否则 budget 会完全绕过钳制直达上游。
		return clampPassthroughBudgetOnly(raw, style, clamp)
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

	// 钳制发生 → 更新 effort 字段
	if clamped == "" {
		slog.Warn("passthrough reasoning effort clamped to empty (stripped)",
			"original", original,
			"style", style,
			"reason", "none_not_in_whitelist")
		return stripPassthroughThinking(raw, style)
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

// clampPassthroughBudgetOnly 处理 passthrough 路径的 budget-only 请求（有 budget 字段、无 effort）。
// 上限口径与 transform 路径共用 transform.BudgetLimitForClamp（白名单最高档），不反推 effort。
//   - 上限为 unconstrained → 原样返回
//   - 上限为 0（白名单禁思考）→ 剥离 thinking（删 effort/budget 字段 + 清理空对象）
//   - budget 超上限 → 钳到上限；低于上限不动
func clampPassthroughBudgetOnly(raw []byte, style string, clamp *transform.ThinkingClampConfig) []byte {
	budgetFields := passthroughBudgetFields(style)
	if len(budgetFields) == 0 {
		return raw // 该协议无 budget 字段（如 OpenAI Chat），无从钳起
	}

	limit := transform.BudgetLimitForClamp(clamp)
	if limit == transform.BudgetLimitUnconstrained {
		return raw
	}

	if limit == 0 {
		slog.Warn("passthrough budget-only reasoning stripped",
			"style", style,
			"reason", "whitelist_has_no_positive_thinking_level")
		return stripPassthroughThinking(raw, style)
	}

	for _, field := range budgetFields {
		budgetVal := gjson.GetBytes(raw, field).Int()
		if budgetVal <= 0 {
			continue
		}
		if budgetVal > limit {
			slog.Warn("passthrough budget-only reasoning budget clamped to whitelist limit",
				"field", field,
				"original_budget", budgetVal,
				"clamped_budget", limit,
				"style", style,
				"reason", "budget_exceeds_whitelist_max_level")
			next, err := sjson.SetBytes(raw, field, limit)
			if err != nil {
				slog.Error("passthrough budget-only clamp: sjson set failed, skipping field",
					"field", field, "error", err, "style", style)
				continue
			}
			raw = next
		}
	}
	return raw
}

// reconcileThinkingBudgetWithMaxTokens 收敛「thinking.budget_tokens >= max_tokens」的非法组合。
// Anthropic 硬约束：budget_tokens 必须严格小于 max_tokens（后者是思考+回答总额），违反必 400。
// 这类非法组合多由 llmux 自己造成——clampMaxTokens 把 max_tokens 压到 MaxTokensLimit 时不看 budget，
// 故本函数必须在 clampMaxTokens **之后**执行，且只降 budget、不抬 max_tokens（运维阀值不能被请求绕过）。
//
// 只在组合已非法时介入（budget >= max_tokens），合法组合原样放行——不二次猜测客户端意图。
// 目标值低于 anthropic.MinThinkingBudget 时钳制无意义（低于最小预算同样 400），整体剥离 thinking。
// 防御式改写：非 anthropic / 字段缺失 / sjson 失败均返回原 body，不阻断主流程。
func reconcileThinkingBudgetWithMaxTokens(body []byte, providerType string) []byte {
	// 只有 Anthropic 有此约束：OpenAI Chat 无 budget 字段；Responses 的 budget 在
	// reasoning.max_tokens、其上限键是 max_output_tokens（不被 clampMaxTokens 触及），不存在此冲突。
	if providerType != consts.StyleAnthropic || len(body) == 0 {
		return body
	}

	budget := gjson.GetBytes(body, "thinking.budget_tokens").Int()
	if budget <= 0 {
		return body
	}

	maxTok := gjson.GetBytes(body, "max_tokens").Int()
	if maxTok <= 0 {
		// max_tokens 缺失时上游用模型默认值，llmux 无模型级上限可回填，直接放行
		// （与 clampMaxTokens 一致：不新增键）。
		return body
	}

	if budget < maxTok {
		return body // 已合法
	}

	target := int64(math.Floor(float64(maxTok) * budgetMaxTokensRatio))
	if target < anthropic.MinThinkingBudget {
		slog.Warn("thinking stripped: max_tokens too small to hold a legal thinking budget",
			"max_tokens", maxTok,
			"original_budget", budget,
			"target_budget", target,
			"min_budget", anthropic.MinThinkingBudget,
			"reason", "budget_exceeds_max_tokens_and_target_below_min")
		return stripPassthroughThinking(body, consts.StyleAnthropic)
	}

	next, err := sjson.SetBytes(body, "thinking.budget_tokens", target)
	if err != nil {
		slog.Error("reconcile thinking budget: sjson set failed, sending original body",
			"error", err, "max_tokens", maxTok, "original_budget", budget)
		return body
	}

	slog.Warn("thinking budget reconciled to fit max_tokens",
		"max_tokens", maxTok,
		"original_budget", budget,
		"clamped_budget", target,
		"reason", "budget_must_be_less_than_max_tokens")
	return next
}

// stripPassthroughThinking 从 raw body 删除该 style 的 effort + budget 字段并清理残留空对象。
// 供 effort 钳成空串（none 剥离）与 budget-only 白名单禁思考两条路径共用。
func stripPassthroughThinking(raw []byte, style string) []byte {
	for _, field := range passthroughEffortFields(style) {
		if next, err := sjson.DeleteBytes(raw, field); err == nil {
			raw = next
		}
	}
	raw = cleanupEmptyPassthroughObjects(raw, style)
	for _, field := range passthroughBudgetFields(style) {
		if next, err := sjson.DeleteBytes(raw, field); err == nil {
			raw = next
		}
	}
	for _, container := range passthroughThinkingContainers(style) {
		if next, err := sjson.DeleteBytes(raw, container); err == nil {
			raw = next
		}
	}
	return cleanupEmptyPassthroughObjects(raw, style)
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
