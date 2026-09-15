package common

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestNormalizeCallerKeyValue 锁 AC-2 规范化规则：
// 去空白、缺前缀补 sk-llmux-、补后最终长度 <16 拒绝（阈值只判最终值——方案 §5）。
func TestNormalizeCallerKeyValue(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{"空串", "", "", false},
		{"纯空白", "   \t ", "", false},
		{"带前缀且足够长原样保留", "sk-llmux-my-codex-key", "sk-llmux-my-codex-key", true},
		{"缺前缀自动补", "my-custom-key-value", "sk-llmux-my-custom-key-value", true},
		{"首尾空白先裁再判", "  sk-llmux-padded-key  ", "sk-llmux-padded-key", true},
		{"裁后缺前缀补", "  custom-outer-key  ", "sk-llmux-custom-outer-key", true},
		{"补前缀后仍不足16拒绝", "ab", "", false},
		{"恰好16通过", "sk-llmux-abcdefg", "sk-llmux-abcdefg", true},
		{"恰好15拒绝", "sk-llmux-abcdef", "", false},
		{"无空格裸值中间空白保留（不做语义清洗）", "my key has space", "sk-llmux-my key has space", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeCallerKeyValue(tc.raw)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("NormalizeCallerKeyValue(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestNormalizeCallerKeyValue_LenientLengths 锁边界不变量：返回 ok=true 的值
// 永远满足「带前缀且长度 ≥16」，调用方可信任契约不再复查。
func TestNormalizeCallerKeyValue_LenientLengths(t *testing.T) {
	for _, raw := range []string{"1234567", "a-very-long-custom-key-value", strings.Repeat("x", 200)} {
		got, ok := NormalizeCallerKeyValue(raw)
		if !ok {
			t.Fatalf(" NormalizeCallerKeyValue(%q) 应通过", raw)
		}
		if !strings.HasPrefix(got, CallerKeyPrefix) || len(got) < MinCallerKeyValueLen {
			t.Fatalf("ok=true 但契约不满足: %q (len=%d)", got, len(got))
		}
	}
}

// TestGenerateCallerKeyValue 锁「省略值时自动生成」（Ruling 2026-09-15）：
// 恒带前缀、长度合规、hex 随机体合法、两次调用不重复。
func TestGenerateCallerKeyValue(t *testing.T) {
	first, err := GenerateCallerKeyValue()
	if err != nil {
		t.Fatalf("GenerateCallerKeyValue: %v", err)
	}
	second, err := GenerateCallerKeyValue()
	if err != nil {
		t.Fatalf("GenerateCallerKeyValue: %v", err)
	}
	if first == second {
		t.Fatal("两次生成结果相同，随机源失效")
	}
	for _, key := range []string{first, second} {
		if !strings.HasPrefix(key, CallerKeyPrefix) {
			t.Fatalf("缺前缀: %q", key)
		}
		if len(key) < MinCallerKeyValueLen {
			t.Fatalf("长度不合规: %q len=%d", key, len(key))
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(key, CallerKeyPrefix)); err != nil {
			t.Fatalf("随机体非 hex: %v", err)
		}
		// 生成值必须能通过自家规范化（幂等）
		normalized, ok := NormalizeCallerKeyValue(key)
		if !ok || normalized != key {
			t.Fatalf("生成值应原样通过规范化: (%q, %v)", normalized, ok)
		}
	}
}
