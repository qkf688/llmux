package chat

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

// Stage 26：请求头快照的自配头名盲区收口——deny-list 只认四个固定键，
// 管理员自配的凭据头（customer_headers / key 级自定义头）键名须动态并入掩码判据。

func modelsRawLogOptionsHeadersOnly() models.RawLogOptions {
	return models.RawLogOptions{RequestHeaders: true}
}

func TestCaptureRequestLogSnapshot_MasksCustomCredentialHeaders(t *testing.T) {
	h := http.Header{
		"Authorization":  {"Bearer sk-fixed"},
		"X-Relay-Token":  {"relay-sk-xxx"},
		"X-Org":          {"org-42"},
		"X-Debug-Header": {"trace-1"},
	}
	snapshot := captureRequestLogSnapshot(
		modelsRawLogOptionsHeadersOnly(),
		h,
		nil, nil, 0,
		[]string{"x-relay-token", "X-Org"}, // 自配头名（允许非 canonical 形态）
	)

	var parsed map[string][]string
	if err := json.Unmarshal(snapshot.RequestHeadersJSON, &parsed); err != nil {
		t.Fatalf("snapshot not valid JSON: %v (%s)", err, snapshot.RequestHeadersJSON)
	}
	if got := parsed["Authorization"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("fixed deny-list key not masked: %q", got)
	}
	if got := parsed["X-Relay-Token"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("custom credential header not masked: %q", got)
	}
	if got := parsed["X-Org"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("custom header (canonical variants in config) not masked: %q", got)
	}
	if got := parsed["X-Debug-Header"][0]; got != "trace-1" {
		t.Fatalf("non-listed header value should be preserved: %q", got)
	}
}

func TestCaptureRequestLogSnapshot_NoExtraKeys_BehaviorUnchanged(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer sk"}, "X-A": {"1"}}
	snapshot := captureRequestLogSnapshot(
		modelsRawLogOptionsHeadersOnly(), h, nil, nil, 0, nil,
	)
	var parsed map[string][]string
	if err := json.Unmarshal(snapshot.RequestHeadersJSON, &parsed); err != nil {
		t.Fatalf("snapshot not valid JSON: %v", err)
	}
	if got := parsed["Authorization"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("Authorization not masked: %q", got)
	}
	if got := parsed["X-A"][0]; got != "1" {
		t.Fatalf("X-A changed: %q", got)
	}
}
