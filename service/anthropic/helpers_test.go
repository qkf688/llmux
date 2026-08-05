package anthropic

import "testing"

func TestThinkingBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		budgetTokens   int64
		expectedEffort string
	}{
		{"high effort", 50000, "high"},
		{"medium effort", 30000, "medium"},
		{"medium lower bound", 20000, "medium"}, // 档位下界恰等值
		{"low effort", 5000, "low"},
		{"zero budget", 0, ""},
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

func TestReasoningEffortToThinkingBudget(t *testing.T) {
	tests := []struct {
		name           string
		effort         string
		expectedBudget int64
	}{
		{"high effort", "high", 50000},
		{"medium effort", "medium", 20000},
		{"low effort", "low", 1000},
		{"empty effort", "", 0},
		{"unknown effort", "super", 0},
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
