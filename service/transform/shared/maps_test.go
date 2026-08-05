package shared

import (
	"context"
	"testing"
)

// Stage B 扩档后 NormalizeReasoningEffort 只做小写归一化，不再回退默认值。
// 6 档 [minimal, low, medium, high, xhigh, max] + 2 特殊 [none, auto] + 未知档位均原样小写透传。
// 钳制由 models.ClampReasoningEffort 在 chat 主路径完成，归一化与钳制职责分离。
//
// Stage A 分界测试演进（P3-1）：Stage A 固化「minimal→low」「未知→low」行为，
// Stage B 扩档后 minimal 保留原值、未知档位原样透传。本测试同步更新为 Stage B 行为。
func TestNormalizeReasoningEffort_StageB_SixLevelsPassthrough(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// 6 档：原样返回（小写归一化）
		{"minimal", "minimal", "minimal"},
		{"low", "low", "low"},
		{"medium", "medium", "medium"},
		{"high", "high", "high"},
		{"xhigh", "xhigh", "xhigh"},
		{"max", "max", "max"},

		// 2 特殊档位：原样返回
		{"none", "none", "none"},
		{"auto", "auto", "auto"},

		// 大写归一化
		{"LOW uppercase", "LOW", "low"},
		{"MINIMAL uppercase", "MINIMAL", "minimal"},
		{"XHIGH uppercase", "XHIGH", "xhigh"},

		// 未知档位：原样小写透传（不再回退默认值，钳制交给 ClampReasoningEffort）
		{"garbage passthrough", "garbage", "garbage"},
		{"empty passthrough", "", ""},
		{"super passthrough", "super", "super"},
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
