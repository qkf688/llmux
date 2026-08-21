package shared

// ThinkingBudgetToReasoningEffort 把「思考 token 预算」粗化成 reasoning effort 档位。
//
// 放在 shared 而非某个协议包：它是**协议中立**的降级换算——入参是任意协议携带的
// token 数（Anthropic 的 thinking.budget_tokens、Responses 的 reasoning.max_tokens），
// 出参是 OpenAI 的 effort 枚举。多个入站协议都需要它把「客户端只给了预算数字」补成
// 「同时也有档位」，避免出站到只认档位的上游时思考意图整体丢失。
//
// 反向的 effort→budget 换算**不在这里**：那条方向的取值（含 minimal/low 合并到 1024）
// 编码的是 Anthropic 的 budget_tokens 硬地板，属于 Anthropic 协议知识，
// 留在 service/anthropic（见该包的 ReasoningEffortToThinkingBudget / MinThinkingBudget）。
//
// 阈值沿用 Octopus 实现（非官方来源）：>=50000→high, >=20000→medium, >0→low，
// 另有 1-512→minimal, 50001-80000→xhigh, 80001+→max。0 与负数返回空串表示「无档位可言」。
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
