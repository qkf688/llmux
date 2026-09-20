package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// 访问行单行上限契约测试（Stage 16）在 access_log_test.go（含自本文件顺迁的
// 换行注入回归用例）。

func TestRecovery_PanicGetsGeneric500WithRidAndSlogTrace(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(RequestID())
	r.Use(Recovery())
	r.GET("/boom", func(c *gin.Context) {
		secret := "sk-super-secret"
		_ = secret
		panic("boom with sk-super-secret")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态 = %d, want 500", w.Code)
	}
	body := w.Body.String()
	rid := w.Header().Get(consts.HeaderRequestID)
	if rid == "" || !strings.Contains(body, "[req="+rid+"]") {
		t.Errorf("panic 型 500 响应缺编号文案（rid=%q）: %s", rid, body)
	}
	if strings.Contains(body, "sk-super-secret") || strings.Contains(body, "goroutine") {
		t.Errorf("panic 文本/栈泄进响应体: %s", body)
	}
	ls := logs.String()
	if !strings.Contains(ls, "panic recovered") || !strings.Contains(ls, "stack=") {
		t.Errorf("panic 未经 slog 留栈:\n%s", ls)
	}
	if !strings.Contains(ls, "request_id="+rid) {
		t.Errorf("panic 日志行缺 rid:\n%s", ls)
	}
	if !strings.Contains(ls, "boom with sk-super-secret") {
		t.Errorf("panic 原文未进日志（日志侧必须保真）:\n%s", ls)
	}
}

// 已提交响应（SSE 半开流）后 panic：不得再追加 JSON body 污染流。
func TestRecovery_AfterCommittedResponseDoesNotAppendBody(t *testing.T) {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(Recovery())
	r.GET("/stream", func(c *gin.Context) {
		c.Writer.WriteString("data: partial\n\n")
		c.Writer.Flush()
		panic("late boom")
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if strings.Contains(w.Body.String(), "internal server error") {
		t.Errorf("已提交流被追加了 JSON: %q", w.Body.String())
	}
}

// ErrAbortHandler 是控制流（主动掐断 io 泵），必须原样再抛——与 gin.Recovery 一致。
func TestRecovery_RepanicsAbortHandler(t *testing.T) {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(Recovery())
	r.GET("/abort", func(c *gin.Context) {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		rec := recover()
		if rec != http.ErrAbortHandler {
			t.Errorf("ErrAbortHandler 未被原样再抛, got %v", rec)
		}
	}()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", nil))
	t.Error("预期 panic 穿透，未发生")
}
