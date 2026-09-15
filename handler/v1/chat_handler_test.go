package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/service"
)

func TestChatHandler_PreprocessError_ReturnsHTTP400(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/chat", func(c *gin.Context) {
		chatHandler(c, func([]byte) (*service.Before, error) {
			return nil, errors.New("boom")
		}, nil, "")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if w.Body.String() == "" {
		t.Fatalf("expected response body, got empty")
	}
}

// newReqMeta 是 CallerKey 快照 → models.ReqMeta 的唯一翻译点：
// 有快照填两字段，无快照（JWT 路径/未挂鉴权的测试路由）保持空态而非零值 id。
func TestNewReqMeta_CallerKeySnapshotTranslation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withCaller, _ := gin.CreateTestContext(httptest.NewRecorder())
	withCaller.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	callerID := uint(9)
	withCaller.Set(middleware.ContextKeyCallerKey, &middleware.CallerKey{ID: callerID, Name: "claude-code"})

	meta := newReqMeta(withCaller)
	if meta.ApiKeyID == nil || *meta.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d", meta.ApiKeyID, callerID)
	}
	if meta.ApiKeyName != "claude-code" {
		t.Fatalf("ApiKeyName = %q, want %q", meta.ApiKeyName, "claude-code")
	}

	withoutCaller, _ := gin.CreateTestContext(httptest.NewRecorder())
	withoutCaller.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	empty := newReqMeta(withoutCaller)
	if empty.ApiKeyID != nil {
		t.Fatalf("ApiKeyID = %d, want nil（无 CallerKey 不得指向 id 0）", *empty.ApiKeyID)
	}
	if empty.ApiKeyName != "" {
		t.Fatalf("ApiKeyName = %q, want 空串", empty.ApiKeyName)
	}
}
