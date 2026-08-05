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
}

// TestClampPassthroughReasoning_Responses 覆盖 Responses passthrough 双路径：
// reasoning.effort + metadata.reasoning_effort 同步钳制。
func TestClampPassthroughReasoning_Responses(t *testing.T) {
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("high → medium 双路径同步", func(t *testing.T) {
		raw := []byte(`{"model":"m","reasoning":{"effort":"high"},"metadata":{"reasoning_effort":"high"}}`)
		result := clampPassthroughReasoning(raw, consts.StyleOpenAIRes, clamp)
		effort1 := gjson.GetBytes(result, "reasoning.effort").String()
		effort2 := gjson.GetBytes(result, "metadata.reasoning_effort").String()
		if effort1 != "medium" {
			t.Errorf("reasoning.effort = %q, want medium", effort1)
		}
		if effort2 != "medium" {
			t.Errorf("metadata.reasoning_effort = %q, want medium", effort2)
		}
	})
}

// TestBuildRequestBodyForProvider_TransformClamp 集成测试：
// transform 路径（OpenAI client → Anthropic provider）钳制 unified.ReasoningEffort。
func TestBuildRequestBodyForProvider_TransformClamp(t *testing.T) {
	ctx := context.Background()
	clamp := &transform.ThinkingClampConfig{
		Levels:          []string{"low", "medium"},
		AutoFallback:    "low",
		UnknownStrategy: "clamp_to_default",
	}

	t.Run("OpenAI high → Anthropic medium（transform 钳制）", func(t *testing.T) {
		raw := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`)
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
