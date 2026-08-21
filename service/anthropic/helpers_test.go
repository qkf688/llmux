package anthropic

import "testing"

func TestThinkingBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		budgetTokens   int64
		expectedEffort string
	}{
		// 保留现有 3 档测试（Octopus 阈值不变）
		{"high effort", 50000, "high"},
		{"medium effort", 30000, "medium"},
		{"medium lower bound", 20000, "medium"}, // 档位下界恰等值
		{"low effort", 5000, "low"},
		{"zero budget", 0, ""},

		// 新增 3 档（Stage B 扩档）
		{"minimal effort", 512, "minimal"},
		{"minimal upper bound", 512, "minimal"},
		{"minimal lower bound", 1, "minimal"},
		{"low above minimal", 513, "low"}, // 513 落入 low 区间
		{"xhigh effort", 80000, "xhigh"},
		{"xhigh lower bound", 50001, "xhigh"},
		{"max effort", 128000, "max"},
		{"max lower bound", 80001, "max"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effort := ThinkingBudgetToReasoningEffort(tt.budgetTokens)
			if effort != tt.expectedEffort {
				t.Errorf("Expected effort '%s', got '%s'", tt.expectedEffort, effort)
			}
		})
	}
}

// TestMapAnthropicErrorType 冻结出站错误 type 的白名单钳制：
// Anthropic 的 error.type 是客户端用来分支的机器可读枚举，写出协议未定义的值
// （如 OpenAI 的 rate_limit_exceeded）等于让客户端拿到无法分支的字符串，
// 故未知值一律兜底 api_error（500 语义，最中性的「上游出错了」）。
func TestMapAnthropicErrorType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 官方白名单原样透传
		{"invalid_request_error", "invalid_request_error", "invalid_request_error"},
		{"authentication_error", "authentication_error", "authentication_error"},
		{"billing_error", "billing_error", "billing_error"},
		{"permission_error", "permission_error", "permission_error"},
		{"not_found_error", "not_found_error", "not_found_error"},
		{"request_too_large", "request_too_large", "request_too_large"},
		{"rate_limit_error", "rate_limit_error", "rate_limit_error"},
		{"api_error", "api_error", "api_error"},
		{"overloaded_error", "overloaded_error", "overloaded_error"},

		// 非白名单兜底
		{"空值兜底", "", "api_error"},
		{"OpenAI 专有 code 兜底", "rate_limit_exceeded", "api_error"},
		{"OpenAI server_error 兜底", "server_error", "api_error"},
		{"大小写不同不算命中", "Rate_Limit_Error", "api_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapAnthropicErrorType(tt.input); got != tt.expected {
				t.Errorf("mapAnthropicErrorType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReasoningEffortToThinkingBudget(t *testing.T) {
	tests := []struct {
		name           string
		effort         string
		expectedBudget int64
	}{
		// 保留现有 3 档测试（Octopus 值不变）
		{"high effort", "high", 50000},
		{"medium effort", "medium", 20000},
		{"low effort", "low", 1000},
		{"empty effort", "", 0},
		{"unknown effort", "super", 0},

		// 新增 3 档（Stage B 扩档）
		{"minimal effort", "minimal", 512},
		{"xhigh effort", "xhigh", 80000},
		{"max effort", "max", 128000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := ReasoningEffortToThinkingBudget(tt.effort)
			if budget != tt.expectedBudget {
				t.Errorf("Expected budget %d, got %d", tt.expectedBudget, budget)
			}
		})
	}
}

// TestReasoningEffortBudgetRoundTrip 覆盖 round-trip 路径：
// request_inbound.go（budget→effort 反推）+ request_outbound.go（effort→budget 正映）同一请求内。
// 标注丢档边界：budget=60000 → 反推 xhigh → 正映 80000（多了 20000 token）。
func TestReasoningEffortBudgetRoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		originalBudget int64
		// 反推：budget → effort
		reverseEffort string
		// 正映：effort → budget（round-trip 结果）
		forwardBudget int64
		// 丢档说明：round-trip 后 budget 与原 budget 的差异
		lossNote string
	}{
		// 精确 round-trip（无丢档）
		{"512 → minimal → 512（精确）", 512, "minimal", 512, "无丢档"},
		{"1000 → low → 1000（精确）", 1000, "low", 1000, "无丢档"},
		{"20000 → medium → 20000（精确）", 20000, "medium", 20000, "无丢档"},
		{"50000 → high → 50000（精确）", 50000, "high", 50000, "无丢档"},
		{"80000 → xhigh → 80000（精确）", 80000, "xhigh", 80000, "无丢档"},
		{"128000 → max → 128000（精确）", 128000, "max", 128000, "无丢档"},

		// 丢档边界：budget 落在两档之间，反推到较高档，正映回更高 budget
		{"60000 → xhigh → 80000（丢 20000）", 60000, "xhigh", 80000, "多了 20000 token"},
		{"513 → low → 1000（丢 487）", 513, "low", 1000, "多了 487 token"},
		{"19999 → low → 1000（丢 18999）", 19999, "low", 1000, "少了 18999 token（反推降档）"},
		{"50001 → xhigh → 80000（丢 29999）", 50001, "xhigh", 80000, "多了 29999 token"},
		{"80001 → max → 128000（丢 47999）", 80001, "max", 128000, "多了 47999 token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effort := ThinkingBudgetToReasoningEffort(tt.originalBudget)
			if effort != tt.reverseEffort {
				t.Errorf("reverse: ThinkingBudgetToReasoningEffort(%d) = %q, want %q",
					tt.originalBudget, effort, tt.reverseEffort)
			}
			budget := ReasoningEffortToThinkingBudget(effort)
			if budget != tt.forwardBudget {
				t.Errorf("forward: ReasoningEffortToThinkingBudget(%q) = %d, want %d (%s)",
					effort, budget, tt.forwardBudget, tt.lossNote)
			}
		})
	}
}
