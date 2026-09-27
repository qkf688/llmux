package chat

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

// Stage 16 契约（写侧）：开启「记录请求头」后落库快照不得含明文凭据——敏感头
// （Authorization/Cookie/x-api-key/Proxy-Authorization）键保留、值整值 ****，
// 非敏感头原样；判据单源在 common.MaskSensitiveLogHeaders（deny-list），
// 本测试锁「captureRequestLogSnapshot 在 marshal 前接线了该判据」这一接缝。
// 响应体形状契约在 handler/logs 的请求头掩码契约测试；存量清洗在 repository 层。

func TestCaptureRequestLogSnapshot_MasksSensitiveHeaders(t *testing.T) {
	header := http.Header{
		"Authorization":       {"Bearer sk-secret"},
		"Cookie":              {"session=abc"},
		"X-Api-Key":           {"sk-gw"},
		"Proxy-Authorization": {"Basic zzz"},
		"X-Custom":            {"keep"},
	}
	snapshot := captureRequestLogSnapshot(models.RawLogOptions{RequestHeaders: true}, header, nil, nil, 4096, nil)

	raw := string(snapshot.RequestHeadersJSON)
	for _, secret := range []string{"sk-secret", "session=abc", "sk-gw", "zzz"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("落库快照明文泄露凭据 %q: %s", secret, raw)
		}
	}

	var parsed http.Header
	if err := json.Unmarshal(snapshot.RequestHeadersJSON, &parsed); err != nil {
		t.Fatalf("快照不是合法头 JSON: %v, raw=%s", err, raw)
	}
	for _, key := range []string{"Authorization", "Cookie", "X-Api-Key", "Proxy-Authorization"} {
		if v := parsed.Values(key); len(v) != 1 || v[0] != common.SensitiveHeaderMask {
			t.Errorf("%s = %v, want [%q]（敏感头键保留、值掩码）", key, v, common.SensitiveHeaderMask)
		}
	}
	if parsed.Get("X-Custom") != "keep" {
		t.Errorf("非敏感头 X-Custom 被改动: %s", raw)
	}
	// 克隆语义：入参与出站透传共享引用，不得被就地掩码污染真实出站头。
	if header.Get("Authorization") != "Bearer sk-secret" {
		t.Errorf("入参 Authorization 被就地修改为 %q", header.Get("Authorization"))
	}
}

func TestCaptureRequestLogSnapshot_HeadersOffWritesNothing(t *testing.T) {
	// 回归：开关关闭时行为零变化（快照零值，落库出口不写 RequestHeaders）。
	header := http.Header{"Authorization": {"Bearer sk-secret"}}
	snapshot := captureRequestLogSnapshot(models.RawLogOptions{}, header, nil, nil, 4096, nil)
	if len(snapshot.RequestHeadersJSON) != 0 {
		t.Fatalf("开关关闭时不应采集头快照, got %s", snapshot.RequestHeadersJSON)
	}
	if header.Get("Authorization") != "Bearer sk-secret" {
		t.Errorf("入参被就地修改: %q", header.Get("Authorization"))
	}
}
