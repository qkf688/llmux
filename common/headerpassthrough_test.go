package common

import (
	"encoding/json"
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

// ---- Stage 16 日志快照脱敏：敏感头 deny-list 值掩码（判据单源）----
// 契约：敏感头键保留、值整值掩码（保留「带没带这个头」的排障信息，值无排障价值）；
// 非敏感头原样；纯函数克隆语义——ReqMeta.Header 与出站透传共享同一引用，
// 就地修改会污染真实出站头。

func TestMaskSensitiveLogHeaders(t *testing.T) {
	in := http.Header{
		"Authorization":       {"Bearer sk-secret"},
		"Cookie":              {"session=abc"},
		"X-Api-Key":           {"sk-gateway"},
		"Proxy-Authorization": {"Basic zzz"},
		"X-Request-Id":        {"r1"},
		"User-Agent":          {"cli/1.0"},
		"Content-Type":        {"application/json"},
	}
	got := MaskSensitiveLogHeaders(in)

	for _, key := range []string{"Authorization", "Cookie", "X-Api-Key", "Proxy-Authorization"} {
		if v := got.Values(key); len(v) != 1 || v[0] != SensitiveHeaderMask {
			t.Errorf("%s = %v, want [%q]（敏感头键保留、值整值掩码）", key, v, SensitiveHeaderMask)
		}
	}
	if got.Get("X-Request-Id") != "r1" || got.Get("User-Agent") != "cli/1.0" || got.Get("Content-Type") != "application/json" {
		t.Errorf("非敏感头被改动: %v", got)
	}
	// 克隆语义：入参不得被就地修改
	if in.Get("Authorization") != "Bearer sk-secret" || in.Get("Cookie") != "session=abc" {
		t.Errorf("入参被就地修改（ReqMeta.Header 与出站透传共享引用，禁止就地改）: %v", in)
	}
	// 幂等：二次掩码结果不变
	again := MaskSensitiveLogHeaders(got)
	for key, values := range got {
		if !sameStringSet(again.Values(key), values) {
			t.Errorf("二次掩码后 %s = %v, want %v（幂等）", key, again.Values(key), values)
		}
	}
}

func TestMaskSensitiveLogHeaders_MasksNonCanonicalRawKeys(t *testing.T) {
	// http.Header 不保证调用方构造方式——map 字面量直写的非规范键也必须命中判据。
	in := http.Header{
		"authorization": {"Bearer sk-secret"},
		"x-api-key":     {"sk-raw"},
		"x-custom":      {"keep"},
	}
	got := MaskSensitiveLogHeaders(in)

	// 输出保留原始键形、仅值掩码（Header.Get 只查 canonical 键，这里必须直读 map 键）
	if v := got["authorization"]; len(v) != 1 || v[0] != SensitiveHeaderMask {
		t.Errorf("非规范键 authorization 未被掩码: %v, got=%v", v, got)
	}
	if v := got["x-api-key"]; len(v) != 1 || v[0] != SensitiveHeaderMask {
		t.Errorf("非规范键 x-api-key 未被掩码: %v, got=%v", v, got)
	}
	if v := got["x-custom"]; len(v) != 1 || v[0] != "keep" {
		t.Errorf("非敏感头 x-custom 被改动: %v, got=%v", v, got)
	}
}

func TestMaskSensitiveLogHeadersJSON(t *testing.T) {
	raw := `{"Authorization":["Bearer sk-secret"],"Cookie":["session=abc"],"X-Api-Key":["sk-gw"],"X-Request-Id":["r1"],"User-Agent":["cli/1.0"]}`
	got, err := MaskSensitiveLogHeadersJSON(raw)
	if err != nil {
		t.Fatalf("MaskSensitiveLogHeadersJSON: %v", err)
	}

	var parsed http.Header
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("掩码产物不是合法头 JSON: %v, raw=%s", err, got)
	}
	for _, key := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		if v := parsed.Values(key); len(v) != 1 || v[0] != SensitiveHeaderMask {
			t.Errorf("%s = %v, want [%q]", key, v, SensitiveHeaderMask)
		}
	}
	if parsed.Get("X-Request-Id") != "r1" || parsed.Get("User-Agent") != "cli/1.0" {
		t.Errorf("非敏感头被改动: %s", got)
	}
	// 幂等：已掩码的快照再清洗零改写（存量清洗可无条件重复执行）
	again, err := MaskSensitiveLogHeadersJSON(got)
	if err != nil {
		t.Fatalf("二次掩码: %v", err)
	}
	if again != got {
		t.Errorf("二次掩码结果变化（幂等破坏）:\n got=%s\nagain=%s", got, again)
	}
}

func TestMaskSensitiveLogHeadersJSON_BadJSONReturnsError(t *testing.T) {
	if _, err := MaskSensitiveLogHeadersJSON(`{"Authorization":`); err == nil {
		t.Fatal("坏 JSON 必须返回带上下文错误（清洗侧据此 slog.Warn 跳过该行，不阻断启动）")
	}
}

func TestMaskSensitiveLogHeadersJSON_EmptyInputNoOp(t *testing.T) {
	got, err := MaskSensitiveLogHeadersJSON("")
	if err != nil || got != "" {
		t.Fatalf("空输入应原样返回零错误, got(%q, %v)", got, err)
	}
}
