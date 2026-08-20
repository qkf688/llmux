package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/transform"
	"github.com/tidwall/gjson"
)

// TestClampPassthroughReasoning_OpenAI 覆盖 OpenAI passthrough 路径钳制：
// effort 不在白名单 → 就近钳制；effort 在白名单 → 透传。
func TestClampPassthroughReasoning_OpenAI(t *testing.T) {
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	tests := []struct {
		name        string
		raw         string
		wantEffort  string
		wantClamped bool
	}{
		{
			name:        "high → medium（就近钳制）",
			raw:         `{"model":"m","reasoning_effort":"high"}`,
			wantEffort:  "medium",
			wantClamped: true,
		},
		{
			name:        "low → 透传（在白名单）",
			raw:         `{"model":"m","reasoning_effort":"low"}`,
			wantEffort:  "low",
			wantClamped: false,
		},
		{
			name:        "无 effort → 不动",
			raw:         `{"model":"m"}`,
			wantEffort:  "",
			wantClamped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampPassthroughReasoning([]byte(tt.raw), consts.StyleOpenAI, clamp)
			got := gjson.GetBytes(result, "reasoning_effort").String()
			if got != tt.wantEffort {
				t.Errorf("reasoning_effort = %q, want %q", got, tt.wantEffort)
			}
		})
	}
}

// TestClampPassthroughReasoning_CaseInsensitive 回归 Warning：
// passthrough 路径未对 effortVal 做小写归一化，客户端传 "HIGH" 时白名单命中失败。
// 修复后应与 transform 路径行为一致——先小写归一化再钳制。
func TestClampPassthroughReasoning_CaseInsensitive(t *testing.T) {
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium", "high"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	tests := []struct {
		name       string
		raw        string
		wantEffort string
	}{
		{
			name:       "HIGH 大写 → 在白名单（小写化后 high）→ 透传为 high",
			raw:        `{"model":"m","reasoning_effort":"HIGH"}`,
			wantEffort: "high",
		},
		{
			name:       "Medium 混合大小写 → 在白名单 → 透传为 medium",
			raw:        `{"model":"m","reasoning_effort":"Medium"}`,
			wantEffort: "medium",
		},
		{
			name:       "MAX 大写 → 不在白名单 → 就近钳制到 high",
			raw:        `{"model":"m","reasoning_effort":"MAX"}`,
			wantEffort: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampPassthroughReasoning([]byte(tt.raw), consts.StyleOpenAI, clamp)
			got := gjson.GetBytes(result, "reasoning_effort").String()
			if got != tt.wantEffort {
				t.Errorf("reasoning_effort = %q, want %q", got, tt.wantEffort)
			}
		})
	}
}

// TestClampPassthroughReasoning_Anthropic 覆盖 Anthropic passthrough 路径：
// output_config.effort 钳制 + thinking.budget_tokens 联动（方案 E）。
func TestClampPassthroughReasoning_Anthropic(t *testing.T) {
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("high → medium + budget 联动", func(t *testing.T) {
		raw := []byte(`{"model":"m","output_config":{"effort":"high"},"thinking":{"budget_tokens":50000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		effort := gjson.GetBytes(result, "output_config.effort").String()
		if effort != "medium" {
			t.Errorf("effort = %q, want medium", effort)
		}
		// budget 联动：medium 对应 20000，原 budget=50000 > 20000 → 钳到 20000
		budget := gjson.GetBytes(result, "thinking.budget_tokens").Int()
		if budget != 20000 {
			t.Errorf("budget = %d, want 20000 (clamped to medium limit)", budget)
		}
	})

	t.Run("low → 透传 + budget 不动", func(t *testing.T) {
		raw := []byte(`{"model":"m","output_config":{"effort":"low"},"thinking":{"budget_tokens":1000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		effort := gjson.GetBytes(result, "output_config.effort").String()
		if effort != "low" {
			t.Errorf("effort = %q, want low (passthrough)", effort)
		}
		budget := gjson.GetBytes(result, "thinking.budget_tokens").Int()
		if budget != 1000 {
			t.Errorf("budget = %d, want 1000 (unchanged)", budget)
		}
	})

	t.Run("none 不支持 → 剥离 effort + budget", func(t *testing.T) {
		raw := []byte(`{"model":"m","output_config":{"effort":"none"},"thinking":{"budget_tokens":5000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		if gjson.GetBytes(result, "output_config.effort").Exists() {
			t.Errorf("output_config.effort should be deleted")
		}
		if gjson.GetBytes(result, "output_config").Exists() {
			t.Errorf("output_config should be cleaned up (empty object)")
		}
		if gjson.GetBytes(result, "thinking.budget_tokens").Exists() {
			t.Errorf("thinking.budget_tokens should be deleted (none strips thinking)")
		}
	})

	t.Run("none 不支持 + thinking.type 存在 → thinking 容器整体删除", func(t *testing.T) {
		raw := []byte(`{"model":"m","output_config":{"effort":"none"},"thinking":{"type":"enabled","budget_tokens":5000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		// 只删 budget_tokens 会残留 {"type":"enabled"}，Anthropic 要求 enabled 必须带 budget_tokens
		if gjson.GetBytes(result, "thinking").Exists() {
			t.Errorf("thinking 容器应整体删除, got %s", result)
		}
	})
}

// TestClampPassthroughReasoning_Responses 覆盖 Responses passthrough 的 effort 钳制。
// Responses 只有一条 effort 控制字段 reasoning.effort；metadata 是客户端自由 KV 标签
// （上游不读其中的控制语义），即使恰好含 reasoning_effort 键也不参与钳制。
func TestClampPassthroughReasoning_Responses(t *testing.T) {
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("high → medium，metadata 同名标签原样保留", func(t *testing.T) {
		raw := []byte(`{"model":"m","reasoning":{"effort":"high"},"metadata":{"reasoning_effort":"high"}}`)
		result := clampPassthroughReasoning(raw, consts.StyleOpenAIRes, clamp)
		if effort := gjson.GetBytes(result, "reasoning.effort").String(); effort != "medium" {
			t.Errorf("reasoning.effort = %q, want medium", effort)
		}
		if tag := gjson.GetBytes(result, "metadata.reasoning_effort").String(); tag != "high" {
			t.Errorf("metadata.reasoning_effort = %q, want high（标签不参与钳制）", tag)
		}
	})

	t.Run("官方字段缺席时不从 metadata 反推 effort", func(t *testing.T) {
		raw := []byte(`{"model":"m","metadata":{"reasoning_effort":"high"}}`)
		result := clampPassthroughReasoning(raw, consts.StyleOpenAIRes, clamp)
		if gjson.GetBytes(result, "reasoning.effort").Exists() {
			t.Errorf("不应凭 metadata 造出 reasoning.effort, got %s", result)
		}
		if tag := gjson.GetBytes(result, "metadata.reasoning_effort").String(); tag != "high" {
			t.Errorf("metadata.reasoning_effort = %q, want high", tag)
		}
	})
}

// TestBuildRequestBodyForProvider_TransformClamp 集成测试：
// transform 路径（OpenAI client → Anthropic provider）钳制 unified.ReasoningEffort。
// fixture 必须显式给足 max_tokens：Anthropic 出站在 max_tokens 缺失时硬填 8192
// （service/anthropic/request_outbound.go:30），会让 medium(20000) 落进
// budget >= max_tokens 的非法区间而触发协议级收敛，掩盖本测试要断的白名单钳制结果。
func TestBuildRequestBodyForProvider_TransformClamp(t *testing.T) {
	ctx := context.Background()
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("OpenAI high → Anthropic medium（transform 钳制）", func(t *testing.T) {
		raw := []byte(`{"model":"m","max_tokens":64000,"messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`)
		result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
			Style:            consts.StyleOpenAI,
			ProviderType:     consts.StyleAnthropic,
			Raw:              raw,
			SupportsThinking: true,
			ThinkingClamp:    clamp,
		})
		if skip || err != nil {
			t.Fatalf("skip=%v err=%v", skip, err)
		}
		// Anthropic 出站用 thinking.budget_tokens，medium → 20000
		budget := gjson.GetBytes(result, "thinking.budget_tokens").Int()
		if budget != 20000 {
			t.Errorf("thinking.budget_tokens = %d, want 20000 (medium)", budget)
		}
	})
}

// TestBuildRequestBodyForProvider_PassthroughClamp 集成测试：
// passthrough 路径（同格式）钳制 raw body effort 字段。
func TestBuildRequestBodyForProvider_PassthroughClamp(t *testing.T) {
	ctx := context.Background()
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("OpenAI passthrough high → medium", func(t *testing.T) {
		raw := []byte(`{"model":"m","reasoning_effort":"high"}`)
		result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
			Style:            consts.StyleOpenAI,
			ProviderType:     consts.StyleOpenAI,
			Raw:              raw,
			SupportsThinking: true,
			ThinkingClamp:    clamp,
		})
		if skip || err != nil {
			t.Fatalf("skip=%v err=%v", skip, err)
		}
		effort := gjson.GetBytes(result, "reasoning_effort").String()
		if effort != "medium" {
			t.Errorf("reasoning_effort = %q, want medium", effort)
		}
	})
}

// TestBuildRequestBodyForProvider_SupportsThinkingFalse_StripsAndNoClamp：
// SupportsThinking=false 时 stripThinkingFields 剥离 thinking，ThinkingClamp 为 nil 不钳制。
func TestBuildRequestBodyForProvider_SupportsThinkingFalse_StripsAndNoClamp(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{"model":"m","reasoning_effort":"high"}`)
	result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            consts.StyleOpenAI,
		ProviderType:     consts.StyleOpenAI,
		Raw:              raw,
		SupportsThinking: false,
		ThinkingClamp:    nil, // supportsThinking=false → 不钳制
	})
	if skip || err != nil {
		t.Fatalf("skip=%v err=%v", skip, err)
	}
	if gjson.GetBytes(result, "reasoning_effort").Exists() {
		t.Errorf("reasoning_effort should be stripped (supportsThinking=false)")
	}
}

// TestBuildRequestBodyForProvider_BudgetNotClampedWhenEffortNotClamped：
// effort 在白名单内（未钳制）→ budget 不动（不误伤）。
func TestBuildRequestBodyForProvider_BudgetNotClampedWhenEffortNotClamped(t *testing.T) {
	ctx := context.Background()
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium", "high"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("Anthropic passthrough high 在白名单 → budget 不动", func(t *testing.T) {
		raw := []byte(`{"model":"m","output_config":{"effort":"high"},"thinking":{"budget_tokens":50000}}`)
		result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
			Style:            consts.StyleAnthropic,
			ProviderType:     consts.StyleAnthropic,
			Raw:              raw,
			SupportsThinking: true,
			ThinkingClamp:    clamp,
		})
		if skip || err != nil {
			t.Fatalf("skip=%v err=%v", skip, err)
		}
		budget := gjson.GetBytes(result, "thinking.budget_tokens").Int()
		if budget != 50000 {
			t.Errorf("budget = %d, want 50000 (effort not clamped, budget unchanged)", budget)
		}
	})
}

// TestClampPassthroughReasoning_BudgetOnly 回归 passthrough 路径的 budget-only 缺口：
// clampPassthroughReasoning 曾在 effortVal 为空时直接 return raw，让 budget 绕过白名单。
// 上限口径与 transform 路径共用（白名单最高档 budget），不反推 effort。
func TestClampPassthroughReasoning_BudgetOnly(t *testing.T) {
	t.Run("Anthropic budget 超白名单上限 → 钳到 medium(20000)", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          []string{"low", "medium"},
			AutoFallback:    "low",
			UnknownStrategy: "clamp_to_default",
		}
		raw := []byte(`{"model":"m","thinking":{"budget_tokens":60000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		if got := gjson.GetBytes(result, "thinking.budget_tokens").Int(); got != 20000 {
			t.Errorf("budget = %d, want 20000", got)
		}
		if gjson.GetBytes(result, "output_config.effort").Exists() {
			t.Errorf("budget-only 不应凭空写出 effort 字段")
		}
	})

	t.Run("Anthropic budget 未超上限 → 不动", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          []string{"low", "medium"},
			AutoFallback:    "low",
			UnknownStrategy: "clamp_to_default",
		}
		raw := []byte(`{"model":"m","thinking":{"budget_tokens":5000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		if got := gjson.GetBytes(result, "thinking.budget_tokens").Int(); got != 5000 {
			t.Errorf("budget = %d, want 5000 (unchanged)", got)
		}
	})

	t.Run("Anthropic 白名单只含 none → 剥离 thinking budget", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          []string{"none"},
			AutoFallback:    "low",
			UnknownStrategy: "clamp_to_default",
		}
		raw := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":5000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		if gjson.GetBytes(result, "thinking.budget_tokens").Exists() {
			t.Errorf("budget 应被剥离")
		}
		// thinking 只剩 {"type":"enabled"} 会被上游判为缺 budget_tokens 而 400，容器须一起删
		if gjson.GetBytes(result, "thinking").Exists() {
			t.Errorf("thinking 容器应整体删除, got %s", result)
		}
	})

	t.Run("Responses budget-only 钳 reasoning.max_tokens", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          []string{"low", "medium"},
			AutoFallback:    "low",
			UnknownStrategy: "clamp_to_default",
		}
		raw := []byte(`{"model":"m","reasoning":{"max_tokens":60000}}`)
		result := clampPassthroughReasoning(raw, consts.StyleOpenAIRes, clamp)
		if got := gjson.GetBytes(result, "reasoning.max_tokens").Int(); got != 20000 {
			t.Errorf("reasoning.max_tokens = %d, want 20000", got)
		}
	})

	t.Run("OpenAI 无 budget 字段 → 原样返回", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          []string{"low"},
			AutoFallback:    "low",
			UnknownStrategy: "clamp_to_default",
		}
		raw := []byte(`{"model":"m"}`)
		result := clampPassthroughReasoning(raw, consts.StyleOpenAI, clamp)
		if string(result) != string(raw) {
			t.Errorf("OpenAI budget-only 应原样返回, got %s", result)
		}
	})

	t.Run("白名单空 + passthrough → budget 不动", func(t *testing.T) {
		clamp := &transform.ThinkingClampConfig{
			Levels:          nil,
			AutoFallback:    "low",
			UnknownStrategy: "passthrough",
		}
		raw := []byte(`{"model":"m","thinking":{"budget_tokens":999999}}`)
		result := clampPassthroughReasoning(raw, consts.StyleAnthropic, clamp)
		if got := gjson.GetBytes(result, "thinking.budget_tokens").Int(); got != 999999 {
			t.Errorf("budget = %d, want 999999 (unconstrained passthrough)", got)
		}
	})
}

// TestBuildRequestBodyForProvider_ABStageBoundary：
// A/B 阶段分界——minimal 在 Stage B 扩档后保留原值（不再映射为 low）。
func TestBuildRequestBodyForProvider_ABStageBoundary_MinimalPreserved(t *testing.T) {
	ctx := context.Background()
	// 白名单含 minimal，minimal 应透传
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"minimal", "low", "medium", "high"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	raw := []byte(`{"model":"m","reasoning_effort":"minimal"}`)
	result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            consts.StyleOpenAI,
		ProviderType:     consts.StyleOpenAI,
		Raw:              raw,
		SupportsThinking: true,
		ThinkingClamp:    clamp,
	})
	if skip || err != nil {
		t.Fatalf("skip=%v err=%v", skip, err)
	}
	effort := gjson.GetBytes(result, "reasoning_effort").String()
	if effort != "minimal" {
		t.Errorf("reasoning_effort = %q, want minimal (Stage B 扩档后保留原值)", effort)
	}
}

// TestReconcileThinkingBudget 覆盖「thinking budget 超 max_tokens」的协议级收敛。
// 背景：clampMaxTokens 把 max_tokens 压到 MaxTokensLimit 时不看 budget_tokens，
// 会自造 budget >= max_tokens 的非法组合（Anthropic 必 400）。方向是降 budget、不抬 max_tokens。
func TestReconcileThinkingBudget(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		raw          string
		wantBudget   int64 // -1 = 期望 thinking 整体不存在
		wantMaxTok   int64
	}{
		{
			name:         "AC-1 budget 超 max → 降到 floor(max*0.8)，max 不抬高",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   6553,
			wantMaxTok:   8192,
		},
		{
			name:         "AC-2 目标值低于最小预算 → 剥离 thinking",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":1000,"output_config":{"effort":"high"},"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   -1,
			wantMaxTok:   1000,
		},
		{
			name:         "AC-3 组合已合法 → budget 不动（不二次猜测客户端意图）",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":4000}}`,
			wantBudget:   4000,
			wantMaxTok:   8192,
		},
		{
			name:         "AC-4 openai → 不触碰",
			providerType: consts.StyleOpenAI,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   50000,
			wantMaxTok:   8192,
		},
		{
			name:         "AC-4 openai-res → 不触碰",
			providerType: consts.StyleOpenAIRes,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   50000,
			wantMaxTok:   8192,
		},
		{
			name:         "AC-5 无 budget 字段 → 原样返回",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":8192}`,
			wantBudget:   0,
			wantMaxTok:   8192,
		},
		{
			name:         "AC-5 budget 为 0 → 原样返回",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":0}}`,
			wantBudget:   0,
			wantMaxTok:   8192,
		},
		{
			name:         "无 max_tokens → 原样返回（不凭空造 max_tokens）",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   50000,
			wantMaxTok:   0,
		},
		{
			name:         "边界 max=1280 → budget=1024 刚好等于最小预算，保留",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":1280,"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   1024,
			wantMaxTok:   1280,
		},
		{
			name:         "边界 max=1279 → budget=1023 低于最小预算，剥离",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":1279,"thinking":{"type":"enabled","budget_tokens":50000}}`,
			wantBudget:   -1,
			wantMaxTok:   1279,
		},
		{
			name:         "budget 恰等于 max → 仍属非法，收敛",
			providerType: consts.StyleAnthropic,
			raw:          `{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":8192}}`,
			wantBudget:   6553,
			wantMaxTok:   8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconcileThinkingBudgetWithMaxTokens([]byte(tt.raw), tt.providerType)

			if tt.wantBudget < 0 {
				if gjson.GetBytes(result, "thinking").Exists() {
					t.Errorf("thinking 容器应整体删除, got %s", result)
				}
				if gjson.GetBytes(result, "output_config.effort").Exists() {
					t.Errorf("output_config.effort 应删除, got %s", result)
				}
			} else if got := gjson.GetBytes(result, "thinking.budget_tokens").Int(); got != tt.wantBudget {
				t.Errorf("thinking.budget_tokens = %d, want %d", got, tt.wantBudget)
			}

			if got := gjson.GetBytes(result, "max_tokens").Int(); got != tt.wantMaxTok {
				t.Errorf("max_tokens = %d, want %d（收敛只降 budget，不动 max_tokens）", got, tt.wantMaxTok)
			}
		})
	}
}

// TestReconcileThinkingBudget_BothPathsAgree 覆盖 AC-6：
// passthrough 与 transform 两条构建路径对等价输入必须产出一致的 budget/max_tokens 组合，
// 防两条路径各写一份实现后行为漂移。
func TestReconcileThinkingBudget_BothPathsAgree(t *testing.T) {
	ctx := context.Background()
	limit := 8192
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium", "high"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	// passthrough：anthropic → anthropic，客户端自带 max_tokens=64000 + budget=50000
	passthroughRaw := []byte(`{"model":"m","max_tokens":64000,"messages":[{"role":"user","content":"hi"}],"output_config":{"effort":"high"},"thinking":{"type":"enabled","budget_tokens":50000}}`)
	passthroughOut, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            consts.StyleAnthropic,
		ProviderType:     consts.StyleAnthropic,
		Raw:              passthroughRaw,
		MaxTokensLimit:   &limit,
		SupportsThinking: true,
		ThinkingClamp:    clamp,
	})
	if skip || err != nil {
		t.Fatalf("passthrough: skip=%v err=%v", skip, err)
	}

	// transform：openai → anthropic，effort=high 映射出 budget=50000
	transformRaw := []byte(`{"model":"m","max_tokens":64000,"messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`)
	transformOut, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            consts.StyleOpenAI,
		ProviderType:     consts.StyleAnthropic,
		Raw:              transformRaw,
		MaxTokensLimit:   &limit,
		SupportsThinking: true,
		ThinkingClamp:    clamp,
	})
	if skip || err != nil {
		t.Fatalf("transform: skip=%v err=%v", skip, err)
	}

	const wantBudget = 6553 // floor(8192*0.8)
	const wantMaxTok = 8192

	for _, c := range []struct {
		path string
		body []byte
	}{{"passthrough", passthroughOut}, {"transform", transformOut}} {
		budget := gjson.GetBytes(c.body, "thinking.budget_tokens").Int()
		maxTok := gjson.GetBytes(c.body, "max_tokens").Int()
		if budget != wantBudget {
			t.Errorf("%s: thinking.budget_tokens = %d, want %d", c.path, budget, wantBudget)
		}
		if maxTok != wantMaxTok {
			t.Errorf("%s: max_tokens = %d, want %d", c.path, maxTok, wantMaxTok)
		}
	}
}

// TestReconcileThinkingBudget_AnthropicDefaultMaxTokens 覆盖不依赖 MaxTokensLimit 的触发路径：
// 客户端不给 max_tokens 时 Anthropic 出站硬填 8192（request_outbound.go:30），
// 而 effort=high 映射出 budget=50000 —— 非法组合由默认值单独造成，与 clampMaxTokens 无关。
func TestReconcileThinkingBudget_AnthropicDefaultMaxTokens(t *testing.T) {
	ctx := context.Background()
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium", "high"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	raw := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`)
	result, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            consts.StyleOpenAI,
		ProviderType:     consts.StyleAnthropic,
		Raw:              raw,
		MaxTokensLimit:   nil, // 无运维上限，非法组合纯由出站默认 max_tokens 造成
		SupportsThinking: true,
		ThinkingClamp:    clamp,
	})
	if skip || err != nil {
		t.Fatalf("skip=%v err=%v", skip, err)
	}
	if got := gjson.GetBytes(result, "max_tokens").Int(); got != 8192 {
		t.Fatalf("max_tokens = %d, want 8192 (Anthropic 出站默认值)", got)
	}
	if got := gjson.GetBytes(result, "thinking.budget_tokens").Int(); got != 6553 {
		t.Errorf("thinking.budget_tokens = %d, want 6553 (floor(8192*0.8))", got)
	}
}
