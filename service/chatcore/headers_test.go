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

	got := BuildHeaders(source, true, nil, map[string]string{
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

	got := BuildHeaders(source, false, nil, map[string]string{
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

	got := BuildHeaders(source, true, nil, nil, false)

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

	got := BuildHeaders(source, true, nil, map[string]string{
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

// TestBuildHeaders_FourLayerPriority 钉方案 §3 的四层链（低→高）：
// 入站白名单透传 < key 级头 < 关联级 customer_headers < providers 权威（第四层发生
// 在 BuildHeaders 之后，由 service/chat 的 e2e 用例打在真实上游收到的头上）。
// 撞名裁决是产品语义不是实现细节：渠道要求（关联级）比客户端伪装（key 级）更具体。
func TestBuildHeaders_FourLayerPriority(t *testing.T) {
	source := http.Header{}
	source.Set("User-Agent", "browser/1.0") // 白名单头：入站值最低层
	source.Set("X-Relay", "inbound-relay")  // 三层撞名的公共基准
	source.Set("X-Inbound-Only", "pass-me") // 只有入站有 → 应存活

	got := BuildHeaders(source, true,
		map[string]string{
			"User-Agent": "codex_cli_rs/0.1", // 覆盖入站
			"X-Relay":    "key-relay",        // 将被关联级再覆盖
			"X-App":      "codex",            // key 独占头
		},
		map[string]string{
			"X-Relay": "association-relay", // 关联级赢
		},
		false)

	if got.Get("User-Agent") != "codex_cli_rs/0.1" {
		t.Errorf("User-Agent = %q, want key 级覆盖入站透传", got.Get("User-Agent"))
	}
	if got.Get("X-Relay") != "association-relay" {
		t.Errorf("X-Relay = %q, want 关联级覆盖 key 级（撞名裁决）", got.Get("X-Relay"))
	}
	if got.Get("X-App") != "codex" {
		t.Errorf("X-App = %q, want key 独占头送达", got.Get("X-App"))
	}
	if got.Get("X-Inbound-Only") != "pass-me" {
		t.Errorf("X-Inbound-Only = %q, want 透传存活", got.Get("X-Inbound-Only"))
	}
}

// key 级头是显式通道：不受白名单限制（Authorization/Cookie 等被入站封死的头
// 管理员仍可经 key 级送达上游，providers 权威覆写在其后兜底）；with_header=false
// 时入站不透传但 key 头照样注入——两开关正交。
func TestBuildHeaders_KeyHeadersBypassWhitelistAndOrthogonalToWithHeader(t *testing.T) {
	source := http.Header{}
	source.Set("Cookie", "session=secret")

	got := BuildHeaders(source, false, map[string]string{"Cookie": "configured-by-key"}, nil, false)
	if got.Get("Cookie") != "configured-by-key" {
		t.Errorf("Cookie = %q, want key 级显式配置送达（不受 with_header/白名单限制）", got.Get("Cookie"))
	}
}

// TestBuildHeaders_StripsAcceptEncodingEvenWhenExplicitlyConfigured 钉「显式通道不设限」
// 的唯一例外——传输层头 Accept-Encoding：
//
//   - 自设它会让 Go Transport 放弃透明解压（判据：transport 只在调用方未设此头时才自动
//     加 gzip 并解压），gzip 上游会被读流侧误判死；
//   - 且上游响应会保留 Content-Encoding，而响应头允许清单（handler/v1）恰好剔除该头，
//     客户端因此拿到「压缩字节 + 无编码标记」——不可解码。
//
// 故三层（入站透传 / key 级 / 关联级）任一路径设了它都要在出口删掉，与 service/credprobe
// 的探针同款；这条是刻意区别于 Cookie/Authorization 等「管理员显式通道照放」的头。
func TestBuildHeaders_StripsAcceptEncodingEvenWhenExplicitlyConfigured(t *testing.T) {
	source := http.Header{}
	source.Set("Accept-Encoding", "gzip")

	got := BuildHeaders(source, true,
		map[string]string{"Accept-Encoding": "br"},
		map[string]string{"Accept-Encoding": "gzip, deflate"},
		false)

	if v := got.Get("Accept-Encoding"); v != "" {
		t.Fatalf("Accept-Encoding = %q, want 空（传输层头不得外发，解压交给 Transport 透明完成）", v)
	}
}
