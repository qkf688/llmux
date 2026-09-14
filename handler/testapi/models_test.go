package testapi

import (
	"net/http"
	"net/textproto"
	"testing"
)

func TestBuildTestHeaders(t *testing.T) {
	source := http.Header{
		"Authorization": []string{"Bearer source-token"},
		"X-Trace-ID":    []string{"trace-1"},
	}
	trueVal := true
	falseVal := false

	tests := []struct {
		name          string
		withHeader    *bool
		customHeaders map[string]string
		wantAuth      string
		wantExtra     string
		wantTrace     string
	}{
		{
			name:          "custom overrides win over passthrough",
			withHeader:    &trueVal,
			customHeaders: map[string]string{"Authorization": "Bearer custom-token", "X-Custom": "v"},
			wantAuth:      "Bearer custom-token",
			wantExtra:     "v",
			// 白名单头在透传分支原样保留（Authorization 不保留，见白名单用例）
			wantTrace: "trace-1",
		},
		{
			name:          "ignore source when disabled",
			withHeader:    &falseVal,
			customHeaders: map[string]string{"X-Custom": "v2"},
			wantAuth:      "",
			wantExtra:     "v2",
			wantTrace:     "",
		},
		{
			name:          "nil withHeader means no source passthrough",
			withHeader:    nil,
			customHeaders: map[string]string{"X-Custom": "v3"},
			wantAuth:      "",
			wantExtra:     "v3",
			wantTrace:     "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := BuildTestHeaders(source, tc.withHeader, tc.customHeaders)
			if got.Get("Authorization") != tc.wantAuth {
				t.Fatalf("Authorization = %q, want %q", got.Get("Authorization"), tc.wantAuth)
			}
			if got.Get("X-Custom") != tc.wantExtra {
				t.Fatalf("X-Custom = %q, want %q", got.Get("X-Custom"), tc.wantExtra)
			}
			if got.Get("X-Trace-ID") != tc.wantTrace {
				t.Fatalf("X-Trace-ID = %q, want %q", got.Get("X-Trace-ID"), tc.wantTrace)
			}
		})
	}
}

// TestBuildTestHeaders_PassThroughWhitelist 是「透传请求头」的外泄面契约：
// with_header=true 只允许白名单头出网，会话材料（Authorization/Cookie）、来源信息
// （Referer/Origin）与传输层头（Accept-Encoding/Connection/Proxy-*/Sec-*）一律不发第三方供应商。
// 背景见 .local/find-issues/透传请求头把浏览器cookie-sent-to-第三方供应商.md。
func TestBuildTestHeaders_PassThroughWhitelist(t *testing.T) {
	source := http.Header{}
	// 会话/身份材料：绝不可外发
	source.Set("Authorization", "Bearer admin-jwt")
	source.Set("Cookie", "session=secret")
	source.Set("X-Api-Key", "gateway-key")
	// 来源与内部结构信息
	source.Set("Referer", "http://internal-host/settings")
	source.Set("Origin", "http://internal-host")
	// 传输层：自设 Accept-Encoding 会禁用 Go Transport 透明解压，好渠道会被判死
	source.Set("Accept-Encoding", "gzip")
	source.Set("Connection", "keep-alive")
	source.Set("Proxy-Authorization", "Basic proxy")
	source.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZQ==")
	source.Set("Host", "spoofed-host")
	// X- 前缀泛放行下必须显式挡掉的 IP/路由变体（对抗性排除分支，删了测试要能报警）；
	// 其中一项刻意用**原始小写 map 键**注入，验证「先规范化再比较」不被绕过。
	source["x-forwarded-for"] = []string{"203.0.113.7"}
	source.Set("X-Real-IP", "203.0.113.8")
	source.Set("X-Client-IP", "203.0.113.9")
	source.Set("X-True-Ip", "203.0.113.10")
	source.Set("X-Remote-Addr", "203.0.113.11")
	source.Set("X-Original-Uri", "/internal/settings")
	// 白名单头：调试用途，应原样保留
	source.Set("User-Agent", "Mozilla/5.0 (test)")
	source.Set("X-Request-Id", "req-42")
	source.Set("X-Trace-Id", "trace-42")
	source.Set("OpenAI-Beta", "assistants=v2")
	source.Set("Anthropic-Beta", "fine-tuning-2024")
	source.Set("Anthropic-Version", "2023-06-01")
	source.Set("X-Organization-Id", "org-9")

	trueVal := true
	got := BuildTestHeaders(source, &trueVal, nil)

	mustNotLeak := []string{
		"Authorization", "Cookie", "X-Api-Key", "Referer", "Origin",
		"Accept-Encoding", "Connection", "Proxy-Authorization",
		"Sec-WebSocket-Key", "Host",
		"X-Forwarded-For", "X-Real-Ip", "X-Client-Ip",
		"X-True-Ip", "X-Remote-Addr", "X-Original-Uri",
	}
	for _, key := range mustNotLeak {
		if _, exists := got[textproto.CanonicalMIMEHeaderKey(key)]; exists {
			t.Errorf("header %q leaked to upstream: %q", key, got.Values(key))
		}
	}

	wantKept := map[string]string{
		"User-Agent":        "Mozilla/5.0 (test)",
		"X-Request-Id":      "req-42",
		"X-Trace-Id":        "trace-42",
		"OpenAI-Beta":       "assistants=v2",
		"Anthropic-Beta":    "fine-tuning-2024",
		"Anthropic-Version": "2023-06-01",
		"X-Organization-Id": "org-9",
	}
	for key, want := range wantKept {
		if gotGet := got.Get(key); gotGet != want {
			t.Errorf("whitelisted header %q = %q, want %q", key, gotGet, want)
		}
	}
}

// TestBuildTestHeaders_CustomerHeadersOverrideWhitelist 保证 customer_headers 这条
// 显式通道不受白名单限制（中转渠道的鉴权头由管理员手配），且后写覆盖透传结果。
func TestBuildTestHeaders_CustomerHeadersOverrideWhitelist(t *testing.T) {
	source := http.Header{}
	source.Set("X-Request-Id", "req-from-browser")
	trueVal := true
	custom := map[string]string{
		"X-Request-Id": "req-custom",
		"X-Auth-Token": "relay-token",
	}

	got := BuildTestHeaders(source, &trueVal, custom)

	if got.Get("X-Request-Id") != "req-custom" {
		t.Errorf("X-Request-Id = %q, want %q", got.Get("X-Request-Id"), "req-custom")
	}
	if got.Get("X-Auth-Token") != "relay-token" {
		t.Errorf("X-Auth-Token = %q, want %q", got.Get("X-Auth-Token"), "relay-token")
	}
}
