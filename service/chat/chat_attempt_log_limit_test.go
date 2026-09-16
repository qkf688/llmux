package chat

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

// capture400KiB 单响应采集上限（Stage 2-D）：只截**落库文本**，绝不截**下游数据流**——
// 采集读头部 limit 字节后必须把完整响应原样交还（MultiReader），否则 transform/直通
// 会读到残缺 JSON（采集是旁路，不是消费方）。
func TestCaptureRawResponseBody_HeadOnlyKeepsTailReadable(t *testing.T) {
	initChatRecordTestDB(t)

	payload := []byte(strings.Repeat("字", (400<<10)/3)) // 400 KiB，256 KiB 处正劈在多字节中间
	res := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(bytes.NewReader(payload)),
	}

	got := captureRawResponseBody(context.Background(), models.RawLogOptions{RawResponseBody: true}, res)

	limit := getRawLogFieldMaxBytes(context.Background())
	if len(got) > limit+len(common.TruncatedSuffix) {
		t.Fatalf("落库文本 %d 字节, want ≤ %d", len(got), limit+len(common.TruncatedSuffix))
	}
	if !strings.HasSuffix(got, common.TruncatedSuffix) {
		t.Fatalf("超限必须带截断标记")
	}
	if !utf8.ValidString(strings.TrimSuffix(got, common.TruncatedSuffix)) {
		t.Fatalf("截断正文劈进多字节字符（U+FFFD 风险）")
	}
	if !strings.HasPrefix(string(payload), got[:len(got)-len(common.TruncatedSuffix)]) {
		t.Fatalf("截断结果不是原文前缀")
	}

	rest, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if !bytes.Equal(rest, payload) {
		t.Fatalf("采集后下游 body 不完整: got %d bytes, want %d", len(rest), len(payload))
	}
}

// attempt 级契约：请求体/原始请求体/原始响应体三处采集文本全部被单字段上限收口，
// 且直通响应完整（超限截断只发生在落库文本上）。
func TestExecuteSingleProviderAttempt_RawLogFieldsCapped(t *testing.T) {
	initChatRecordTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		Update("value", `{"raw_request_body":true,"raw_response_body":true}`).Error; err != nil {
		t.Fatalf("set log raw options: %v", err)
	}

	reqRaw := `{"model":"m1","messages":[{"role":"user","content":"` + strings.Repeat("图", 150<<10) + `"}]}` // ~450KB
	respPayload := `{"id":"chatcmpl-c","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"` + strings.Repeat("答", 130<<10) + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, respPayload)
	}))
	t.Cleanup(srv.Close)

	providerName := "p-logcap"
	result := runAttemptAgainstURL(t, providerName, srv.URL, srv.Client(), context.Background(), reqRaw)
	if !result.Success {
		t.Fatalf("attempt 应成功: %+v", result)
	}
	defer result.Response.Body.Close()
	full, err := io.ReadAll(result.Response.Body)
	if err != nil {
		t.Fatalf("read downstream body: %v", err)
	}
	if string(full) != respPayload {
		t.Fatalf("直通响应被采集截断破坏: got %d bytes, want %d", len(full), len(respPayload))
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", providerName).First(&row).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	limit := getRawLogFieldMaxBytes(context.Background())
	for name, got := range map[string]string{"RawRequestBody": row.RawRequestBody, "RawResponseBody": row.RawResponseBody} {
		if len(got) > limit+len(common.TruncatedSuffix) {
			t.Fatalf("%s %d 字节, want ≤ %d", name, len(got), limit+len(common.TruncatedSuffix))
		}
		if !strings.HasSuffix(got, common.TruncatedSuffix) {
			t.Fatalf("%s 应被截断并带标记（源数据 ~450KB > 上限）", name)
		}
		if !utf8.ValidString(strings.TrimSuffix(got, common.TruncatedSuffix)) {
			t.Fatalf("%s 截断劈进多字节字符", name)
		}
	}
}
