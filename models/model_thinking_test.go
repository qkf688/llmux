package models

import (
	"context"
	"path/filepath"
	"testing"
)

// TestClampReasoningEffort 表驱动覆盖 ClampReasoningEffort 全部分支：
// 白名单空（unknown_strategy 两策略）、白名单非空（精确命中/none/auto/就近钳制平手偏低/未知档位）。
func TestClampReasoningEffort(t *testing.T) {
	tests := []struct {
		name            string
		effort          string
		levels          []string
		autoFallback    string
		unknownStrategy string
		expected        string
	}{
		// effort 空串 → 原样返回空串
		{"empty effort → empty", "", []string{"low"}, "low", "clamp_to_default", ""},

		// 白名单空 + clamp_to_default：已知 6 档透传，未知档位兜底 autoFallback
		{"empty levels + clamp + low", "low", nil, "low", "clamp_to_default", "low"},
		{"empty levels + clamp + minimal", "minimal", nil, "low", "clamp_to_default", "minimal"},
		{"empty levels + clamp + max", "max", nil, "low", "clamp_to_default", "max"},
		{"empty levels + clamp + unknown → fallback", "garbage", nil, "medium", "clamp_to_default", "medium"},
		{"empty levels + clamp + none → fallback（none 非 6 档）", "none", nil, "low", "clamp_to_default", "low"},
		{"empty levels + clamp + auto → fallback（auto 非 6 档）", "auto", nil, "high", "clamp_to_default", "high"},
		{"empty levels + clamp + unknown + empty fallback → low", "garbage", nil, "", "clamp_to_default", "low"},

		// 白名单空 + passthrough：原样透传
		{"empty levels + passthrough + low", "low", nil, "low", "passthrough", "low"},
		{"empty levels + passthrough + unknown", "garbage", nil, "low", "passthrough", "garbage"},
		{"empty levels + passthrough + none", "none", nil, "low", "passthrough", "none"},
		{"empty levels + passthrough + auto", "auto", nil, "low", "passthrough", "auto"},

		// 白名单非空 + 精确命中 → 透传
		{"exact match low", "low", []string{"low", "high"}, "low", "clamp_to_default", "low"},
		{"exact match max", "max", []string{"minimal", "max"}, "low", "clamp_to_default", "max"},
		{"exact match none", "none", []string{"none", "low"}, "low", "clamp_to_default", "none"},
		{"exact match auto", "auto", []string{"auto", "low"}, "low", "clamp_to_default", "auto"},

		// none 不在白名单 → 剥离（空串）
		{"none not in whitelist → strip", "none", []string{"low", "high"}, "low", "clamp_to_default", ""},

		// auto 不在白名单 → 取白名单最低 6 档
		{"auto not in whitelist → lowest", "auto", []string{"medium", "high"}, "low", "clamp_to_default", "medium"},
		{"auto not in whitelist → lowest minimal", "auto", []string{"minimal", "high"}, "low", "clamp_to_default", "minimal"},
		// 白名单只含 none（无 6 档）→ 兜底 autoFallback
		{"auto not in whitelist + only none → fallback", "auto", []string{"none"}, "high", "clamp_to_default", "high"},

		// 就近钳制（平手偏低）：找白名单中 <= effort 的最高档
		{"clamp high to medium（平手偏低）", "high", []string{"low", "medium"}, "low", "clamp_to_default", "medium"},
		{"clamp xhigh to high", "xhigh", []string{"low", "high"}, "low", "clamp_to_default", "high"},
		{"clamp max to xhigh", "max", []string{"low", "xhigh"}, "low", "clamp_to_default", "xhigh"},
		// effort 是白名单最低档 → 无更低档，取白名单最低档（=effort 本身，因精确命中已先判）
		// 这里 effort=low 不在白名单 [medium, high]，无更低档 → 取最低 medium
		{"clamp low to whitelist lowest（无更低档）", "low", []string{"medium", "high"}, "low", "clamp_to_default", "medium"},
		// effort=minimal 不在白名单 [medium, max]，无更低档 → 取最低 medium
		{"clamp minimal to whitelist lowest", "minimal", []string{"medium", "max"}, "low", "clamp_to_default", "medium"},

		// 未知档位 + 白名单非空 → 兜底白名单最低档
		{"unknown + non-empty whitelist → lowest", "garbage", []string{"low", "high"}, "low", "clamp_to_default", "low"},
		// 白名单只含 none/auto（无 6 档）+ 未知档位 → 兜底 autoFallback
		{"unknown + whitelist only none → fallback", "garbage", []string{"none"}, "medium", "clamp_to_default", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampReasoningEffort(tt.effort, tt.levels, tt.autoFallback, tt.unknownStrategy)
			if got != tt.expected {
				t.Errorf("ClampReasoningEffort(%q, %v, %q, %q) = %q, want %q",
					tt.effort, tt.levels, tt.autoFallback, tt.unknownStrategy, got, tt.expected)
			}
		})
	}
}

// TestThinkingLevelsGORMSerialization 验证 *[]string 三态（nil/空切片/非空）的 GORM 存取行为。
// 这是 I3 审查项：*[]string 三态序列化无先例，需单元测试固化契约。
func TestThinkingLevelsGORMSerialization(t *testing.T) {
	Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test-thinking.db"))
	t.Cleanup(func() {
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	strSlicePtr := func(v []string) *[]string { return &v }

	// 1) nil（继承）→ 存取后保持 nil
	mwpNil := ModelWithProvider{
		ModelID: 1, ProviderID: 1, ProviderModel: "pm-nil",
		ThinkingLevels: nil,
	}
	if err := DB.Create(&mwpNil).Error; err != nil {
		t.Fatalf("create nil: %v", err)
	}
	var gotNil ModelWithProvider
	if err := DB.First(&gotNil, mwpNil.ID).Error; err != nil {
		t.Fatalf("reload nil: %v", err)
	}
	if gotNil.ThinkingLevels != nil {
		t.Errorf("nil case: ThinkingLevels = %v, want nil", gotNil.ThinkingLevels)
	}

	// 2) 空切片（显式不约束）→ 存取后保持空切片（非 nil）
	mwpEmpty := ModelWithProvider{
		ModelID: 1, ProviderID: 1, ProviderModel: "pm-empty",
		ThinkingLevels: strSlicePtr([]string{}),
	}
	if err := DB.Create(&mwpEmpty).Error; err != nil {
		t.Fatalf("create empty: %v", err)
	}
	var gotEmpty ModelWithProvider
	if err := DB.First(&gotEmpty, mwpEmpty.ID).Error; err != nil {
		t.Fatalf("reload empty: %v", err)
	}
	if gotEmpty.ThinkingLevels == nil {
		t.Errorf("empty case: ThinkingLevels = nil, want non-nil empty slice")
	}
	if len(*gotEmpty.ThinkingLevels) != 0 {
		t.Errorf("empty case: ThinkingLevels len = %d, want 0", len(*gotEmpty.ThinkingLevels))
	}

	// 3) 非空（override 白名单）→ 存取后保持内容
	mwpNonEmpty := ModelWithProvider{
		ModelID: 1, ProviderID: 1, ProviderModel: "pm-nonempty",
		ThinkingLevels: strSlicePtr([]string{"low", "high", "max"}),
	}
	if err := DB.Create(&mwpNonEmpty).Error; err != nil {
		t.Fatalf("create non-empty: %v", err)
	}
	var gotNonEmpty ModelWithProvider
	if err := DB.First(&gotNonEmpty, mwpNonEmpty.ID).Error; err != nil {
		t.Fatalf("reload non-empty: %v", err)
	}
	if gotNonEmpty.ThinkingLevels == nil {
		t.Fatalf("non-empty case: ThinkingLevels = nil, want non-nil")
	}
	want := []string{"low", "high", "max"}
	if !sliceEqual(*gotNonEmpty.ThinkingLevels, want) {
		t.Errorf("non-empty case: ThinkingLevels = %v, want %v", *gotNonEmpty.ThinkingLevels, want)
	}
}

// TestHighestEffortInWhitelist 覆盖 budget-only 上限口径的档位查询：
// 取白名单中最高 6 档；不含 6 档（空白名单 / 只含 none-auto）返回空串=不允许思考。
func TestHighestEffortInWhitelist(t *testing.T) {
	tests := []struct {
		name     string
		levels   []string
		expected string
	}{
		{"nil levels → empty", nil, ""},
		{"empty slice → empty", []string{}, ""},
		{"single level", []string{"medium"}, "medium"},
		{"multiple → highest", []string{"low", "medium"}, "medium"},
		{"unordered input → highest", []string{"high", "minimal", "medium"}, "high"},
		{"full 6 levels → max", []string{"minimal", "low", "medium", "high", "xhigh", "max"}, "max"},
		{"only none → empty", []string{"none"}, ""},
		{"only none+auto → empty", []string{"none", "auto"}, ""},
		{"none+auto mixed with 6-level → highest 6-level", []string{"none", "auto", "low"}, "low"},
		{"unknown garbage only → empty", []string{"garbage"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HighestEffortInWhitelist(tt.levels); got != tt.expected {
				t.Errorf("HighestEffortInWhitelist(%v) = %q, want %q", tt.levels, got, tt.expected)
			}
		})
	}
}
