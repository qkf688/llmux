package common

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestExtractUpstreamErrorDetail 锁上游错误细节提取的单一来源语义：
// 沿 error/message 字段递归追最内层人读文本（含「message 本身又是 JSON 信封」的网关套娃），
// 追不到则回退原文，一律受截断上限保护。
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
		{
			// 网关（opencode go 实弹形状）把 provider 原始错误体 JSON 编码成字符串塞进 message
			name: "message is itself a json envelope unwraps to innermost",
			body: `{"type":"error","error":{"type":"error","error":{"type":"UsageLimitError","message":"5 hour usage limit reached. Resets in 3hr 30min.","metadata":{"limitName":"5 hour"}}}}`,
			want: "5 hour usage limit reached. Resets in 3hr 30min.",
		},
		{
			name: "string error field containing json unwraps",
			body: `{"error":"{\"error\":{\"message\":\"upstream rejected key\"}}"}`,
			want: "upstream rejected key",
		},
		{
			name: "top level message without error wrapper",
			body: `{"type":"overloaded_error","message":"Overloaded"}`,
			want: "Overloaded",
		},
		{
			name: "bare string error field is the message",
			body: `{"error":"quota exhausted"}`,
			want: "quota exhausted",
		},
		{
			name: "json string error field without inner message stays that string",
			body: `{"error":"{\"code\":123}"}`,
			want: `{"code":123}`,
		},
		{
			// 病态自嵌套不会无限递归：耗尽层数预算后回退原文
			name: "pathological self nesting bounded by depth budget",
			body: selfNestedErrorEnvelope(upstreamUnwrapMaxDepth + 3),
			want: selfNestedErrorEnvelope(upstreamUnwrapMaxDepth + 3),
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

// selfNestedErrorEnvelope 构造 depth 层 {"error":{…}} 套娃且全程无可提取 message：
// 用于锁「递归耗尽层数预算后安全回退原文」，而非栈爆炸或返回半成品。
func selfNestedErrorEnvelope(depth int) string {
	body := `{"error":{"code":500}}`
	for range depth {
		body = `{"error":` + body + `}`
	}
	return body
}

func TestUpstreamErrorDetailCarriesStatusCode(t *testing.T) {
	got := UpstreamErrorDetail(429, []byte(`{"error":{"message":"rate limited"}}`))
	want := "rate limited\nHTTP 状态码: 429"
	if got != want {
		t.Fatalf("UpstreamErrorDetail() = %q, want %q", got, want)
	}
}

// TestReadCappedBody 锁 ReadCappedBody 的「上限 × EOF」边界契约：超限截到上限、恰好等于上限/
// 小于上限/空 reader 原样读出。这几档正是截断与 EOF 交互最易被后续改动（误写 limit±1、换原语）
// 回归的点，用表驱动钉死长度与原样性。
func TestReadCappedBody(t *testing.T) {
	limit := upstreamDiagnosticReadLimitBytes
	small := []byte(`{"id":"cmpl-1"}`)
	tests := []struct {
		name    string
		input   []byte
		wantLen int
	}{
		{"oversized truncated to limit", bytes.Repeat([]byte("x"), limit+4096), limit},
		{"exactly limit reads all", bytes.Repeat([]byte("y"), limit), limit},
		{"limit minus one reads all", bytes.Repeat([]byte("z"), limit-1), limit - 1},
		{"small body reads all", small, len(small)},
		{"empty body reads empty", []byte{}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadCappedBody(bytes.NewReader(tc.input))
			if err != nil {
				t.Fatalf("读取失败: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Fatalf("读出长度 = %d, want %d", len(got), tc.wantLen)
			}
			// 未超限的用例内容必须原样；超限用例只保证长度（前缀由 LimitReader 语义保证）。
			if len(tc.input) <= limit && !bytes.Equal(got, tc.input) {
				t.Fatalf("未超限应原样读出: got %q", got)
			}
		})
	}
}

func TestReadCappedBodyPropagatesReadError(t *testing.T) {
	sentinel := errors.New("read boom")
	if _, err := ReadCappedBody(alwaysErrorReader{sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("读取错误必须原样上抛（否则 timeout 归类漏回 network）: got %v", err)
	}
}

type alwaysErrorReader struct{ err error }

func (r alwaysErrorReader) Read([]byte) (int, error) { return 0, r.err }
