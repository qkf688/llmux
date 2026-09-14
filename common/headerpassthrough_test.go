package common

import (
	"net/http"
	"testing"
)

func TestPassthroughAllowedHeader(t *testing.T) {
	tests := []struct {
		canonicalKey string
		want         bool
	}{
		// 显式放行：调试性/协议性头
		{"User-Agent", true},
		{"Openai-Beta", true},
		{"Anthropic-Beta", true},
		{"Anthropic-Version", true},
		{"X-Request-Id", true},
		{"X-Organization-Id", true},
		// 排除：会话与凭据
		{"Authorization", false},
		{"Cookie", false},
		{"X-Api-Key", false},
		{"Proxy-Authorization", false},
		// 排除：来源与内部结构
		{"Referer", false},
		{"Origin", false},
		{"Host", false},
		// 排除：传输层（自设会禁用 Go Transport 透明解压）
		{"Accept-Encoding", false},
		{"Connection", false},
		{"Content-Type", false},
		{"Accept", false},
		// 排除：X- 泛放行下的反代 IP/路由变体（逐名挡）
		{"X-Forwarded-For", false},
		{"X-Forwarded-Proto", false},
		{"X-Real-Ip", false},
		{"X-Client-Ip", false},
		{"X-True-Ip", false},
		{"X-Remote-Addr", false},
		{"X-Original-Uri", false},
		// 排除：非 X- 前缀的普通头默认拒绝
		{"From", false},
		{"Date", false},
	}
	for _, tc := range tests {
		if got := PassthroughAllowedHeader(tc.canonicalKey); got != tc.want {
			t.Errorf("PassthroughAllowedHeader(%q) = %v, want %v", tc.canonicalKey, got, tc.want)
		}
	}
}

func TestMergePassthroughHeaders(t *testing.T) {
	dst := http.Header{}
	src := http.Header{
		"Cookie":          []string{"session=secret"},
		"x-forwarded-for": []string{"203.0.113.7"}, // 原始小写键：须先规范化再判定，且不得以该形合入
		"x-request-id":    []string{"lower-r1"},    // 放行侧正向钉死：非规范键须以规范形入 dst
		"X-Request-Id":    []string{"r1", "r2"},    // 多值追加
		"User-Agent":      []string{"cli/1.0"},
	}
	MergePassthroughHeaders(dst, src)

	if _, exists := dst["Cookie"]; exists {
		t.Errorf("Cookie leaked")
	}
	if _, exists := dst["x-forwarded-for"]; exists {
		t.Errorf("non-canonical leak key bypassed canonicalization")
	}
	if _, exists := dst["X-Forwarded-For"]; exists {
		t.Errorf("X-Forwarded-For leaked")
	}
	if _, exists := dst["x-request-id"]; exists {
		t.Errorf("non-canonical allow key entered dst in raw form")
	}
	// src map 迭代无序，r1/r2 与 lower-r1 的合入先后不确定——只断言值集合
	if got := dst.Values("X-Request-Id"); !sameStringSet(got, []string{"r1", "r2", "lower-r1"}) {
		t.Errorf("X-Request-Id = %v, want 规范形合入且值集合 {r1 r2 lower-r1}", got)
	}
	if dst.Get("User-Agent") != "cli/1.0" {
		t.Errorf("User-Agent = %q, want %q", dst.Get("User-Agent"), "cli/1.0")
	}
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(want))
	for _, value := range want {
		counts[value]++
	}
	for _, value := range got {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func TestMergePassthroughHeaders_AppendsOntoExistingDst(t *testing.T) {
	dst := http.Header{"X-Request-Id": []string{"preset"}}
	src := http.Header{"X-Request-Id": []string{"inbound"}}
	MergePassthroughHeaders(dst, src)

	if got := dst.Values("X-Request-Id"); len(got) != 2 || got[0] != "preset" {
		t.Errorf("merge must append without dropping dst values, got %v", got)
	}
}
