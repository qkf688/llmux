package common

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		max      int
		wantSame bool
		wantLen  int // 截断后（不含标记）的正文字节数
	}{
		{"短于上限原样", "hello", 10, true, 0},
		{"恰好等于上限原样", "hello", 5, true, 0},
		{"ASCII 超限", strings.Repeat("a", 20), 10, false, 10},
		{"上限劈在多字节中间须回退 rune 边界", strings.Repeat("字", 10), 4, false, 3},
		{"上限 0 只剩标记", "abc", 0, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateUTF8(tc.in, tc.max)
			if tc.wantSame {
				if got != tc.in {
					t.Fatalf("未超限必须原样, got %q", got)
				}
				return
			}
			if !strings.HasSuffix(got, TruncatedSuffix) {
				t.Fatalf("超限必须带截断标记, got %q", got)
			}
			body := strings.TrimSuffix(got, TruncatedSuffix)
			if len(body) != tc.wantLen {
				t.Fatalf("正文字节数 = %d, want %d", len(body), tc.wantLen)
			}
			// 核心防线：正文必须停在 rune 边界——劈半的字节进 JSON 会变 U+FFFD 乱码尾。
			if !utf8.ValidString(body) {
				t.Fatalf("截断正文非合法 UTF-8: %q", body)
			}
			if strings.ContainsRune(body, utf8.RuneError) {
				t.Fatalf("截断正文含 U+FFFD: %q", body)
			}
			if !strings.HasPrefix(tc.in, body) {
				t.Fatalf("截断结果不是原文前缀")
			}
		})
	}
}
