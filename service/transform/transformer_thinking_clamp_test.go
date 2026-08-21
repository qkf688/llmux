package transform

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/anthropic"
	"github.com/tidwall/gjson"
)

// TestClampUnifiedReasoning_BudgetOnly 回归「只给 reasoning budget、不给 effort」时
// budget 绕过白名单钳制的缺口：clampUnifiedReasoning 曾在 effort == nil 时直接 return。
//
// 上限口径 = 白名单最高档对应的 budget（不把 budget 反推成 effort——反推有损，
// 513..19999 全塌到 low，会把白名单本已允许的中等 budget 过度降级）。
func TestClampUnifiedReasoning_BudgetOnly(t *testing.T) {
	tests := []struct {
		name            string
		budget          int64
		levels          []string
		autoFallback    string
		unknownStrategy string
		wantStripped    bool // 期望 budget 被剥离（设 nil）
		wantBudget      int64
	}{
		// 白名单最高档 medium(20000) 作上限
		{"over limit → clamped to whitelist max", 60000, []string{"low", "medium"}, "low", "clamp_to_default", false, 20000},
		{"under limit → unchanged", 5000, []string{"low", "medium"}, "low", "clamp_to_default", false, 5000},
		{"exactly at limit → unchanged", 20000, []string{"low", "medium"}, "low", "clamp_to_default", false, 20000},

		// 反推有损的反例：budget 15000 反推是 low(地板 1024)，但白名单允许 medium(20000)，不该被钳
		{"lossy-inference trap → not clamped", 15000, []string{"low", "medium"}, "low", "clamp_to_default", false, 15000},

		// 白名单最高档决定上限的其余档位
		{"whitelist max → limit 128000", 999999, []string{"max"}, "low", "clamp_to_default", false, 128000},
		// minimal/low 的上限即 Anthropic 协议地板 1024（此前为 512，是个非法上限）
		{"whitelist minimal only → limit MinThinkingBudget", 5000, []string{"minimal"}, "low", "clamp_to_default", false, anthropic.MinThinkingBudget},

		// 白名单不含任何正向 6 档 → 剥离 thinking
		{"whitelist only none → stripped", 5000, []string{"none"}, "low", "clamp_to_default", true, 0},
		{"whitelist only none+auto → stripped", 5000, []string{"none", "auto"}, "low", "clamp_to_default", true, 0},

		// 白名单空：按 unknown_strategy 分流（与 ClampReasoningEffort 口径一致）
		{"empty whitelist + passthrough → untouched", 999999, nil, "low", "passthrough", false, 999999},
		{"empty whitelist + clamp_to_default → autoFallback limit", 999999, nil, "low", "clamp_to_default", false, anthropic.MinThinkingBudget},
		{"empty whitelist + clamp_to_default + high fallback", 999999, nil, "high", "clamp_to_default", false, 50000},
		{"empty whitelist + garbage fallback → low limit", 999999, nil, "garbage", "clamp_to_default", false, anthropic.MinThinkingBudget},

		// 非正 budget 不处理（客户端给 0/负值，交由出站协议自行忽略）
		{"zero budget → untouched", 0, []string{"low"}, "low", "clamp_to_default", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := tt.budget
			unified := &models.UnifiedRequest{
				Model:           "m",
				Messages:        []models.UnifiedMessage{{Role: "user", Content: "hi"}},
				ReasoningBudget: &budget,
			}
			clamp := &ThinkingClampConfig{
				Levels:          tt.levels,
				AutoFallback:    tt.autoFallback,
				UnknownStrategy: tt.unknownStrategy,
			}

			clampUnifiedReasoning(unified, clamp, consts.StyleOpenAIRes, consts.StyleAnthropic)

			if unified.ReasoningEffort != nil {
				t.Errorf("ReasoningEffort = %q, want nil（budget-only 不写回 effort）", *unified.ReasoningEffort)
			}
			if tt.wantStripped {
				if unified.ReasoningBudget != nil {
					t.Errorf("ReasoningBudget = %d, want nil（白名单禁止思考应剥离）", *unified.ReasoningBudget)
				}
				return
			}
			if unified.ReasoningBudget == nil {
				t.Fatalf("ReasoningBudget = nil, want %d", tt.wantBudget)
			}
			if *unified.ReasoningBudget != tt.wantBudget {
				t.Errorf("ReasoningBudget = %d, want %d", *unified.ReasoningBudget, tt.wantBudget)
			}
		})
	}
}

// TestClampUnifiedReasoning_EffortPresentUnaffected 钉死 effort 非 nil 时仍走原有路径：
// effort 就近钳制 + budget 联动，不被 budget-only 分支截胡。
func TestClampUnifiedReasoning_EffortPresentUnaffected(t *testing.T) {
	effort := "high"
	budget := int64(60000)
	unified := &models.UnifiedRequest{
		Model:           "m",
		Messages:        []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		ReasoningEffort: &effort,
		ReasoningBudget: &budget,
	}
	clamp := &ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	clampUnifiedReasoning(unified, clamp, consts.StyleOpenAI, consts.StyleAnthropic)

	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "medium" {
		t.Fatalf("ReasoningEffort = %v, want medium", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 20000 {
		t.Fatalf("ReasoningBudget = %v, want 20000", unified.ReasoningBudget)
	}
}

// TestProcessRequest_BudgetOnlyClamped 走 ProcessRequest 全链路验证缺口已闭合：
// responses 入站是唯一能产出 budget-only 统一请求的路径（reasoning.max_tokens 与
// reasoning.effort 完全解耦），出站 anthropic 的 thinking.budget_tokens 应被钳到白名单上限。
func TestProcessRequest_BudgetOnlyClamped(t *testing.T) {
	raw := []byte(`{"model":"m","input":"hi","reasoning":{"max_tokens":60000}}`)
	tm := NewTransformerManager(consts.StyleOpenAIRes, consts.StyleAnthropic)
	clamp := &ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	got, err := tm.ProcessRequest(context.Background(), raw, clamp)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	budget := gjson.GetBytes(got, "thinking.budget_tokens")
	if !budget.Exists() {
		t.Fatalf("thinking.budget_tokens missing, body = %s", got)
	}
	if budget.Int() != 20000 {
		t.Errorf("thinking.budget_tokens = %d, want 20000, body = %s", budget.Int(), got)
	}
}
