package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
)

// TestExecuteSingleProviderAttempt_FailsOn204EmptyBody 锁通过集的边界（Stage 5 审查回批）：
// 204「成功但无正文」不得判成功。直通路径（客户端与上游协议形状相同）没有任何解析步骤，
// 判通过就等于把空 body 直接回给调用方，并走一整套成功副作用——权重/优先级上调、失败
// 计数清零，坏网关反而被加权。这里断言它走的是非 OK 出口（日志 error + 状态码原文）。
func TestExecuteSingleProviderAttempt_FailsOn204EmptyBody(t *testing.T) {
	initChatRecordTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	result := runAttemptAgainstURL(t, "p-204", srv.URL, srv.Client(), context.Background(), smallReqRaw)

	if result.Success {
		t.Fatalf("204 空正文不应判成功, got %+v", result)
	}
	if result.Response != nil {
		t.Fatalf("非 OK 出口应已关闭并丢弃响应, got %+v", result.Response)
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-204").First(&row).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if row.Status != "error" || !strings.Contains(row.Error, "status: 204") {
		t.Fatalf("日志行 = %q/%q, want error + 状态码原文", row.Status, row.Error)
	}
}

// TestExecuteSingleProviderAttempt_PassesOn201 锁真实聊天主路径的通过口径（Stage 5）。
// 放宽前这条路径对 201 走 handleNonOKProviderResponse：日志记 error、按状态码归类冷却
// 凭据/衰减权重、再换下一家重试。放宽后回 201 的兼容网关必须与回 200 完全同路。
// 断言分两层：attempt 结果（Success/Response 状态码）+ 落库行（status=success、error 为空），
// 只断其一测不出「走错分支但结果凑巧对」。
func TestExecuteSingleProviderAttempt_PassesOn201(t *testing.T) {
	initChatRecordTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"chatcmpl-201","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(srv.Close)

	result := runAttemptAgainstURL(t, "p-201", srv.URL, srv.Client(), context.Background(), smallReqRaw)

	if !result.Success {
		t.Fatalf("201 应走成功分支, got %+v", result)
	}
	if result.CredentialFailure || result.RemoveWeight || result.RemovePriority {
		t.Fatalf("201 不得被记成任何级别的失败: %+v", result)
	}
	if result.Response == nil || result.Response.StatusCode != http.StatusCreated {
		t.Fatalf("Response = %+v, want 非空且状态码 201", result.Response)
	}
	if result.Response.Body != nil {
		_ = result.Response.Body.Close()
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-201").First(&row).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if row.Status != "success" || row.Error != "" {
		t.Fatalf("日志行 = %q/%q, want success/空（走非 OK 分支会被改成 error + 状态码原文）", row.Status, row.Error)
	}
}
