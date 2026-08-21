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

			clampUnifiedReasoning(unified, clamp, consts.FormatOpenAIResponses, consts.FormatAnthropic)

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

	clampUnifiedReasoning(unified, clamp, consts.FormatOpenAIChat, consts.FormatAnthropic)

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
	tm := NewTransformerManager(consts.FormatOpenAIResponses, consts.FormatAnthropic)
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

// TestProcessRequest_BudgetOnly_OutboundShape 锁定「responses 入站 budget-only + 白名单钳制」
// 在**每个出站协议**上的 body 形状（上面的 TestProcessRequest_BudgetOnlyClamped 只覆盖 anthropic 出站）。
//
// 为什么断言必须打在出站 body 上：曾被否掉的方案（responses 入站把 budget 反推成 effort
// 写进 unified）让全部既有测试保持绿——clampUnifiedBudgetOnly 的测试直接构造 unified，
// 跨协议转换测试又不传 clamp，改动正好落在两组测试的缝里。只有断出站 body 才能同时抓到
// 「该 emit 的没 emit」与「凭空多 emit 一个客户端从未给过的键」。
func TestProcessRequest_BudgetOnly_OutboundShape(t *testing.T) {
	const budgetOnlyRaw = `{"model":"m","input":"hi","reasoning":{"max_tokens":20000}}`

	// 白名单只含 minimal → budget 上限 = minimal 正推值 = Anthropic 协议地板 1024
	minimalOnly := &ThinkingClampConfig{
		Levels:          []string{"minimal"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}
	noneOnly := &ThinkingClampConfig{
		Levels:          []string{"none"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	tests := []struct {
		name           string
		raw            string
		upstreamFormat consts.WireFormat
		clamp          *ThinkingClampConfig
		want           map[string]any // gjson path → 期望值（int64 / string）
		wantMissing    []string       // 必须整键缺失的 gjson path（区别于「键存在但为 null」）
	}{
		{
			name:           "responses 出站：budget 钳到白名单上限，不凭空补 effort",
			raw:            budgetOnlyRaw,
			upstreamFormat: consts.FormatOpenAIResponses,
			clamp:          minimalOnly,
			want:           map[string]any{"reasoning.max_tokens": anthropic.MinThinkingBudget},
			wantMissing:    []string{"reasoning.effort"},
		},
		{
			// OpenAI chat-completions 没有任何思考 token 字段，budget 只能降级成档位。
			// 期望 low 而非 minimal：反推阈值是 1..512→minimal、>512→low，而 minimal 与 low
			// 的正推 budget 同为 1024，于是钳到 1024 后反推出的档位跳到 low——
			// 即出站降级档位不受白名单约束。这是**已知泄漏而非设计意图**，此处只冻结现状；
			// 要不要让降级结果过一遍白名单是独立议题（会改变真实出站行为，须真机验证）。
			name:           "openai 出站：钳制后的 budget 降级成档位（档位不受白名单约束）",
			raw:            budgetOnlyRaw,
			upstreamFormat: consts.FormatOpenAIChat,
			clamp:          minimalOnly,
			want:           map[string]any{"reasoning_effort": "low"},
			wantMissing:    []string{"reasoning.max_tokens", "thinking.budget_tokens"},
		},
		{
			name:           "白名单只含 none：responses 出站整块剥离 thinking",
			raw:            budgetOnlyRaw,
			upstreamFormat: consts.FormatOpenAIResponses,
			clamp:          noneOnly,
			wantMissing:    []string{"reasoning", "reasoning.max_tokens", "reasoning.effort"},
		},
		{
			name:           "白名单只含 none：openai 出站不 emit 任何档位",
			raw:            budgetOnlyRaw,
			upstreamFormat: consts.FormatOpenAIChat,
			clamp:          noneOnly,
			wantMissing:    []string{"reasoning_effort"},
		},
		{
			name:           "白名单空 + passthrough：budget 原样透传且仍不补 effort",
			raw:            `{"model":"m","input":"hi","reasoning":{"max_tokens":999999}}`,
			upstreamFormat: consts.FormatOpenAIResponses,
			clamp: &ThinkingClampConfig{
				Levels:          nil,
				AutoFallback:    "low",
				UnknownStrategy: "passthrough",
			},
			want:        map[string]any{"reasoning.max_tokens": int64(999999)},
			wantMissing: []string{"reasoning.effort"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := NewTransformerManager(consts.FormatOpenAIResponses, tt.upstreamFormat)
			got, err := tm.ProcessRequest(context.Background(), []byte(tt.raw), tt.clamp)
			if err != nil {
				t.Fatalf("ProcessRequest failed: %v", err)
			}

			for path, exp := range tt.want {
				res := gjson.GetBytes(got, path)
				if !res.Exists() {
					t.Errorf("%s 键缺失, body = %s", path, got)
					continue
				}
				switch want := exp.(type) {
				case int64:
					if res.Int() != want {
						t.Errorf("%s = %d, want %d, body = %s", path, res.Int(), want, got)
					}
				case string:
					if res.String() != want {
						t.Errorf("%s = %q, want %q, body = %s", path, res.String(), want, got)
					}
				default:
					t.Fatalf("用例期望值类型 %T 不受支持", exp)
				}
			}

			for _, path := range tt.wantMissing {
				if res := gjson.GetBytes(got, path); res.Exists() {
					t.Errorf("%s 应整键缺失, 实际 = %v, body = %s", path, res.Value(), got)
				}
			}
		})
	}
}
