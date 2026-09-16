package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/service"
	"github.com/tidwall/gjson"
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
	withCaller.Set(middleware.ContextKeyCallerKey, &middleware.CallerKey{
		ID:   callerID,
		Name: "claude-code",
		// Headers 已由 AuthAPIKey 在鉴权边界浅拷贝成请求私有份；翻译点是**传引用**
		// （同一 map），定格不变量的真正防线是"鉴权处单次拷贝 + 下游只读"，
		// 不是此处二次拷贝。见 chat.go:newReqMeta 注释。
		Headers: map[string]string{"User-Agent": "claude-cli/1.0.0"},
	})

	meta := newReqMeta(withCaller)
	if meta.ApiKeyID == nil || *meta.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d", meta.ApiKeyID, callerID)
	}
	if meta.ApiKeyName != "claude-code" {
		t.Fatalf("ApiKeyName = %q, want %q", meta.ApiKeyName, "claude-code")
	}
	if meta.Headers["User-Agent"] != "claude-cli/1.0.0" {
		t.Fatalf("Headers = %v, want User-Agent=claude-cli/1.0.0", meta.Headers)
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
	if empty.Headers != nil {
		t.Fatalf("Headers = %v, want nil（无 CallerKey 不伪造空 map）", empty.Headers)
	}
}

// Stage 2 资源防御：入站体超 consts.MaxInboundBodyBytes → 真实 413 + 既有 /v1 错误信封
// 形状（code/message），且在预处理之前拦截——超大体没有解析的价值。
// 边界用例锁 off-by-one：恰好等于上限必须放行（走到 preProcessor）。
func TestChatHandler_RequestBodySizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		size       int
		wantStatus int
	}{
		{"超限1字节即拒", consts.MaxInboundBodyBytes + 1, http.StatusRequestEntityTooLarge},
		{"恰等于上限放行到预处理", consts.MaxInboundBodyBytes, http.StatusBadRequest}, // 400 来自 boom，证明未被体积闸误杀
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/chat", func(c *gin.Context) {
				chatHandler(c, func(body []byte) (*service.Before, error) {
					if len(body) > consts.MaxInboundBodyBytes {
						t.Errorf("preProcessor 收到超限请求体: %d 字节", len(body))
					}
					return nil, errors.New("boom")
				}, nil, "")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(strings.Repeat("a", tc.size)))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus == http.StatusRequestEntityTooLarge {
				// 契约打在响应体 JSON 上：信封 code 与 HTTP 状态同值、message 非空。
				if got := gjson.Get(w.Body.String(), "code").Int(); got != http.StatusRequestEntityTooLarge {
					t.Fatalf("信封 code = %d, want %d", got, http.StatusRequestEntityTooLarge)
				}
				if !gjson.Get(w.Body.String(), "message").Exists() {
					t.Fatalf("信封缺 message 字段: %s", w.Body.String())
				}
			}
		})
	}
}
