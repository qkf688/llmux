package transform

// Thinking 思考配置 (Anthropic Extended Thinking)
// 参考: https://docs.anthropic.com/claude/docs/extended-thinking
// 参考: E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\thinking.go
//
// Extended Thinking 功能允许模型在生成响应前进行更深入的推理。
// 通过设置 thinking 配置，可以控制模型的推理行为和预算。
//
// 使用场景:
// - 复杂问题求解
// - 需要深度推理的任务
// - 多步骤逻辑推导
//
// 参数映射:
// - thinking.budget_tokens → reasoning_effort
//   - >= 50000 tokens → "high"
//   - >= 20000 tokens → "medium"
//   - > 0 tokens → "low"
type Thinking struct {
	// Type 思考类型
	// "enabled" - 启用思考功能
	// "disabled" - 禁用思考功能
	Type string `json:"type"`

	// BudgetTokens 推理预算 token 数
	// 控制模型可以使用多少 token 进行推理
	BudgetTokens int64 `json:"budget_tokens"`
}

// thinkingBudgetToReasoningEffort 将 thinking budget 转换为 reasoning effort
// 参考 Octopus 实现的映射规则
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

// reasoningEffortToThinkingBudget 将 reasoning effort 转换为 thinking budget
// 参考 Octopus 实现的映射规则
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
