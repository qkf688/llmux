package shared

import (
	"context"
	"testing"
)

// Stage A/B 分界测试：固化 Stage A 单独上线时 NormalizeReasoningEffort 仍只认 3 档。
// minimal → low（现状行为），xhigh/max/none/auto → 回退默认值（low）。
// Stage B 扩档后这些行为才改变，分界测试确保 A 不越界。
func TestNormalizeReasoningEffort_StageA_OnlyThreeLevels(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// 已知 3 档：原样返回（小写归一化）
		{"low", "low", "low"},
		{"medium", "medium", "medium"},
		{"high", "high", "high"},
		{"LOW uppercase", "LOW", "low"},

		// minimal → low（现状行为）
		{"minimal maps to low", "minimal", "low"},

		// 未知档位（含 Stage B 将扩的 5 档）→ 回退默认值 low
		{"xhigh falls back to default", "xhigh", "low"},
		{"max falls back to default", "max", "low"},
		{"none falls back to default", "none", "low"},
		{"auto falls back to default", "auto", "low"},
		{"empty falls back to default", "", "low"},
		{"garbage falls back to default", "garbage", "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeReasoningEffort(ctx, tt.input)
			if got != tt.want {
				t.Fatalf("NormalizeReasoningEffort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
