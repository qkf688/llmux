package common

import (
	"strings"
	"testing"
)

// TestExtractUpstreamErrorDetail 锁上游错误细节提取的单一来源语义：
// 两种错误信封形状优先取 message，解析不出回退原文，一律受截断上限保护。
// 前三个用例自 handler/testapi 的 TestParseProviderErrorDetail 迁入（迁移不断网）。
func TestExtractUpstreamErrorDetail(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested error message",
			body: `{"error":{"message":"bad key"}}`,
			want: "bad key",
		},
		{
			name: "nested error field",
			body: `{"error":{"error":"provider error"}}`,
			want: "provider error",
		},
		{
			name: "fallback raw body",
			body: `plain error`,
			want: "plain error",
		},
		{
			name: "valid json without error envelope falls back to raw",
			body: `{"detail":"not found"}`,
			want: `{"detail":"not found"}`,
		},
		{
			name: "empty body yields empty",
			body: ``,
			want: "",
		},
		{
			name: "parsed message is truncated too",
			body: `{"error":{"message":"` + strings.Repeat("x", upstreamDetailMaxBytes+100) + `"}}`,
			want: strings.Repeat("x", upstreamDetailMaxBytes) + "…(truncated)",
		},
		{
			name: "oversized raw body truncated",
			body: strings.Repeat("y", upstreamDetailMaxBytes+1),
			want: strings.Repeat("y", upstreamDetailMaxBytes) + "…(truncated)",
		},
		{
			// 512 不是 3 的倍数：按字节切会落在「错」字（3 字节）中间，须回退到 rune 边界
			name: "cjk truncation lands on rune boundary not mid-rune",
			body: strings.Repeat("错", 300), // 900 bytes
			want: strings.Repeat("错", upstreamDetailMaxBytes/3) + "…(truncated)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractUpstreamErrorDetail([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("ExtractUpstreamErrorDetail() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUpstreamErrorDetailCarriesStatusCode(t *testing.T) {
	got := UpstreamErrorDetail(429, []byte(`{"error":{"message":"rate limited"}}`))
	want := "rate limited\nHTTP 状态码: 429"
	if got != want {
		t.Fatalf("UpstreamErrorDetail() = %q, want %q", got, want)
	}
}
