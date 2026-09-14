package chatcore

import (
	"net/http"
	"net/textproto"
	"testing"
)

func TestBuildHeaders_WithHeaderAndCustom(t *testing.T) {
	source := http.Header{}
	source.Set("Authorization", "Bearer secret")
	source.Set("X-Api-Key", "abc")
	source.Set("X-Trace", "trace-1")
	source.Set("X-Custom", "from-source")

	got := BuildHeaders(source, true, map[string]string{
		"X-Custom": "from-custom",
		"X-New":    "new-value",
	}, true)

	if got.Get("Authorization") != "" {
		t.Fatalf("expected Authorization removed, got %q", got.Get("Authorization"))
	}
	if got.Get("X-Api-Key") != "" {
		t.Fatalf("expected X-Api-Key removed, got %q", got.Get("X-Api-Key"))
	}
	if got.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("expected stream header set, got %q", got.Get("X-Accel-Buffering"))
	}
	if got.Get("X-Trace") != "trace-1" {
		t.Fatalf("expected whitelisted header X-Trace passed through, got %q", got.Get("X-Trace"))
	}
	if got.Get("X-Custom") != "from-custom" {
		t.Fatalf("expected custom header override, got %q", got.Get("X-Custom"))
	}
	if got.Get("X-New") != "new-value" {
		t.Fatalf("expected custom header X-New, got %q", got.Get("X-New"))
	}

	// source must stay unchanged.
	if source.Get("Authorization") != "Bearer secret" {
		t.Fatalf("expected source header unchanged, got %q", source.Get("Authorization"))
	}
}

func TestBuildHeaders_WithoutSourceClone(t *testing.T) {
	source := http.Header{}
	source.Set("X-Trace", "trace-1")

	got := BuildHeaders(source, false, map[string]string{
		"X-Custom": "from-custom",
	}, false)

	if got.Get("X-Trace") != "" {
		t.Fatalf("expected source header not cloned, got %q", got.Get("X-Trace"))
	}
	if got.Get("X-Custom") != "from-custom" {
		t.Fatalf("expected custom header set, got %q", got.Get("X-Custom"))
	}
}

// TestBuildHeaders_PassThroughWhitelist 是生产热路径的外泄面契约：with_header=true
// 只允许白名单头出网（判据与 testapi 共用 common.PassthroughAllowedHeader，单一数据源）。
// 背景与口径调研见 .local/find-issues/网关热路径chatcore整包透传入站头给上游.md。
func TestBuildHeaders_PassThroughWhitelist(t *testing.T) {
	source := http.Header{}
	// 会话/凭据材料：绝不可外发
	source.Set("Authorization", "Bearer user-token")
	source.Set("Cookie", "session=secret")
	source.Set("X-Api-Key", "gateway-key")
	// 来源与内部结构信息
	source.Set("Referer", "http://internal-host/chat")
	source.Set("Origin", "http://internal-host")
	// 传输层：自设 Accept-Encoding 会禁用 Go Transport 透明解压 → gzip 上游被误判死
	source.Set("Accept-Encoding", "gzip")
	source.Set("Connection", "keep-alive")
	source.Set("Proxy-Authorization", "Basic proxy")
	source.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZQ==")
	source.Set("Host", "spoofed-host")
	source.Set("Content-Type", "text/plain")
	// X- 泛放行下必须显式挡掉的 IP/路由变体；一项用原始小写 map 键注入，
	// 验证「先规范化再比较」不被绕过。
	source["x-forwarded-for"] = []string{"203.0.113.7"}
	source.Set("X-Real-IP", "203.0.113.8")
	source.Set("X-Client-IP", "203.0.113.9")
	// 白名单头：应原样透传
	source.Set("User-Agent", "codex_cli_rs/0.1")
	source.Set("X-Request-Id", "req-42")
	source.Set("Anthropic-Beta", "fine-tuning-2024")
	source.Set("X-Organization-Id", "org-9")

	got := BuildHeaders(source, true, nil, false)

	mustNotLeak := []string{
		"Authorization", "Cookie", "X-Api-Key", "Referer", "Origin",
		"Accept-Encoding", "Connection", "Proxy-Authorization",
		"Sec-WebSocket-Key", "Host", "Content-Type",
		"X-Forwarded-For", "X-Real-Ip", "X-Client-Ip",
	}
	for _, key := range mustNotLeak {
		if _, exists := got[textproto.CanonicalMIMEHeaderKey(key)]; exists {
			t.Errorf("header %q leaked to upstream: %q", key, got.Values(key))
		}
	}

	wantKept := map[string]string{
		"User-Agent":        "codex_cli_rs/0.1",
		"X-Request-Id":      "req-42",
		"Anthropic-Beta":    "fine-tuning-2024",
		"X-Organization-Id": "org-9",
	}
	for key, want := range wantKept {
		if gotGet := got.Get(key); gotGet != want {
			t.Errorf("whitelisted header %q = %q, want %q", key, gotGet, want)
		}
	}
}

// TestBuildHeaders_CustomHeadersBypassWhitelist 保证 customer_headers 显式通道不受
// 白名单限制（中转渠道鉴权头由管理员手配），且后写覆盖透传结果——与 testapi 同语义。
func TestBuildHeaders_CustomHeadersBypassWhitelist(t *testing.T) {
	source := http.Header{}
	source.Set("X-Request-Id", "req-from-client")

	got := BuildHeaders(source, true, map[string]string{
		"X-Request-Id":  "req-custom",
		"Authorization": "Bearer relay-key",
	}, false)

	if got.Get("X-Request-Id") != "req-custom" {
		t.Errorf("X-Request-Id = %q, want %q", got.Get("X-Request-Id"), "req-custom")
	}
	if got.Get("Authorization") != "Bearer relay-key" {
		t.Errorf("custom Authorization = %q, want %q（显式配置必须可达上游，保旧语义）",
			got.Get("Authorization"), "Bearer relay-key")
	}
}
