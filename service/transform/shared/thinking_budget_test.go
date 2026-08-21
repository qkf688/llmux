package shared

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

		// 负数与 0 同义：无档位可言，调用方据此不补 effort
		{"negative budget", -1, ""},
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
