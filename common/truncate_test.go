package common

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "within limit unchanged", in: "hello", max: 10, want: "hello"},
		{name: "exact limit unchanged", in: "hello", max: 5, want: "hello"},
		{name: "ascii cut appends suffix", in: "hello world", max: 5, want: "hello" + TruncatedSuffix},
		{name: "multibyte no rune split", in: "你好世界", max: 8, want: "你好" + TruncatedSuffix}, // 3 字节/字，8B 处回退到 6B
		{name: "zero limit suffix only", in: "abc", max: 0, want: TruncatedSuffix},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateUTF8(tt.in, tt.max); got != tt.want {
				t.Fatalf("TruncateUTF8(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestTruncateHeadTailUTF8(t *testing.T) {
	t.Run("within limit unchanged", func(t *testing.T) {
		s := "hello"
		if got := TruncateHeadTailUTF8(s, 10, 10); got != s {
			t.Fatalf("got %q, want %q", got, s)
		}
	})

	t.Run("exact sum unchanged", func(t *testing.T) {
		s := "abcdef"
		if got := TruncateHeadTailUTF8(s, 3, 3); got != s {
			t.Fatalf("got %q, want %q", got, s)
		}
	})

	t.Run("ascii head tail with marker", func(t *testing.T) {
		s := strings.Repeat("a", 100)
		got := TruncateHeadTailUTF8(s, 10, 10)
		if !strings.HasPrefix(got, strings.Repeat("a", 10)) {
			t.Fatalf("head not preserved: %q", got[:30])
		}
		if !strings.HasSuffix(got, strings.Repeat("a", 10)) {
			t.Fatalf("tail not preserved: %q", got[len(got)-30:])
		}
		if !strings.Contains(got, "内容过长已截断") || !strings.Contains(got, "省略 80 字节") {
			t.Fatalf("marker missing or wrong: %q", got)
		}
	})

	t.Run("multibyte rune boundaries intact", func(t *testing.T) {
		s := strings.Repeat("你", 100) // 3 bytes each
		got := TruncateHeadTailUTF8(s, 10, 10)
		if !utf8.ValidString(got) {
			t.Fatalf("invalid utf8: %q", got)
		}
		// 10B 处回退到 9B（3 个完整字）
		if !strings.HasPrefix(got, strings.Repeat("你", 3)) {
			t.Fatalf("head wrong: %q", got[:20])
		}
		if !strings.HasSuffix(got, strings.Repeat("你", 3)) {
			t.Fatalf("tail wrong: %q", got[len(got)-20:])
		}
	})

	t.Run("zero head keeps tail only", func(t *testing.T) {
		s := strings.Repeat("b", 50)
		got := TruncateHeadTailUTF8(s, 0, 10)
		if strings.TrimSuffix(got, s[len(s)-10:]) != TruncateHeadTail(0, 10, 40) {
			t.Fatalf("tail-only shape wrong: %q", got)
		}
	})

	t.Run("head tail sum over length unchanged", func(t *testing.T) {
		s := "short"
		if got := TruncateHeadTailUTF8(s, 10, 10); got != s {
			t.Fatalf("got %q, want %q", got, s)
		}
	})
}
