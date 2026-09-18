package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	authservice "github.com/qkf688/llmux/service/auth"
	"gorm.io/gorm"
)

// newPasswordRouter 建一个只挂 ChangePassword 的引擎，context 里预置已登录用户
// （ChangePassword 从 middleware.CurrentUser 取用户，测试直连 handler 时需自行注入）。
func newPasswordRouter(t *testing.T, user *models.User) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/password", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUser, user)
		ChangePassword(c)
	})
	return r
}

func doChangePassword(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bodyMessage(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("响应体不是 JSON: %v (%s)", err, w.Body.String())
	}
	return payload.Message
}

// TestChangePassword_WrongOldPasswordIsRequestError 锁定「旧密码填错 ≠ 凭据失效」：
// 管理端前端把真实 401 一律当登录过期（清会话 + 跳登录页），若这里回 401，
// 管理员打错一次旧密码就会被踢下线，且刚写的提示随跳转消失。
func TestChangePassword_WrongOldPasswordIsRequestError(t *testing.T) {
	hash, err := authservice.HashPassword("correct-old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &models.User{Model: gorm.Model{ID: 1}, Username: "admin", PasswordHash: hash}
	r := newPasswordRouter(t, user)

	w := doChangePassword(t, r, `{"old_password":"wrong","new_password":"new-password-123"}`)

	// 现实 HTTP 状态码必须是 200：前端全局惯例把真实 401 当登录过期。
	// 分类放进信封 code（httpresp.BadRequest 的形状），正是「请求内容不对」。
	if w.Code != http.StatusOK {
		t.Fatalf("旧密码错误应回 HTTP 200 + 信封 code，实得 HTTP %d", w.Code)
	}
	if got := bodyMessage(t, w); got != "old password incorrect" {
		t.Fatalf("message = %q", got)
	}
}

// TestChangePassword_Unauthenticated 未登录（context 无用户）仍回 401——
// 那是真正的凭据失效，前端踢回登录页是正确行为。
func TestChangePassword_Unauthenticated(t *testing.T) {
	r := newPasswordRouter(t, nil)

	w := doChangePassword(t, r, `{"old_password":"x","new_password":"y"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应回 401，实得 HTTP %d", w.Code)
	}
}
