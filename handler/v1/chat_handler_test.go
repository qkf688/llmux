package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
