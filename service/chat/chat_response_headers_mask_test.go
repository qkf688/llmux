package chat

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// Stage 26：响应头快照落库脱敏（判据单源 common）——成功路径（updateRequestAndResponseLog）
// 与失败出口共用序列化单点（responseHeadersJSON）两侧同验。

func TestUpdateRequestAndResponseLog_MasksResponseHeaders(t *testing.T) {
	initChatRecordTestDB(t)

	log := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m-resp-mask", Status: "success", Style: "openai"})

	resHeader := http.Header{
		"Content-Type": {"application/json"},
		"Set-Cookie":   {"sid=leaky"},
	}
	updateRequestAndResponseLog(t.Context(), log.ID, models.RawLogOptions{ResponseHeaders: true}, requestLogSnapshot{}, resHeader, "")

	var row models.ChatLog
	if err := models.DB.First(&row, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.ResponseHeaders == "" {
		t.Fatal("response headers not persisted")
	}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(row.ResponseHeaders), &parsed); err != nil {
		t.Fatalf("response headers not valid JSON: %v (%s)", err, row.ResponseHeaders)
	}
	// 契约打在落库 JSON 上：Set-Cookie 值掩码、Content-Type 原样。
	if got := parsed["Set-Cookie"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("response Set-Cookie not masked: %q", got)
	}
	if got := parsed["Content-Type"][0]; got != "application/json" {
		t.Fatalf("non-sensitive response header value changed: %q", got)
	}
}

func TestResponseHeadersJSON_MasksSensitiveKeys(t *testing.T) {
	raw := string(responseHeadersJSON(http.Header{"Set-Cookie": {"sid=x"}, "X-Ok": {"v"}}))
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v (%s)", err, raw)
	}
	if got := parsed["Set-Cookie"][0]; got != common.SensitiveHeaderMask {
		t.Fatalf("Set-Cookie not masked: %q", got)
	}
	if got := parsed["X-Ok"][0]; got != "v" {
		t.Fatalf("non-sensitive header changed: %q", got)
	}
}
