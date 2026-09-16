package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

func init() { gin.SetMode(gin.TestMode) }

// newRequestIDRouter 建一个只挂 RequestID 中间件 + 回显 context 编号的探针路由。
func newRequestIDRouter() *gin.Engine {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/probe", func(c *gin.Context) {
		v, _ := c.Get(consts.ContextKeyRequestID)
		c.String(http.StatusOK, "%v", v)
	})
	return r
}

func doProbe(t *testing.T, r *gin.Engine, inbound string, hasHeader bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if hasHeader {
		req.Header.Set(consts.HeaderRequestID, inbound)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	w := doProbe(t, newRequestIDRouter(), "", false)

	rid := w.Header().Get(consts.HeaderRequestID)
	if rid == "" {
		t.Fatal("响应头必须带生成的请求编号")
	}
	if len(rid) != 12 {
		t.Errorf("生成的编号长度 = %d, want 12 (%q)", len(rid), rid)
	}
	if body := w.Body.String(); body != rid {
		t.Errorf("context 中的编号 = %q, 与响应头 %q 不一致", body, rid)
	}
}

// 调用方自带规矩的编号 → 沿用之（外部 trace 体系能直接对上服务端日志），
// 且必须回显在响应头，用户报障时拿得到同一个号。
func TestRequestID_ReusesValidInbound(t *testing.T) {
	for _, in := range []string{
		"req_abc123",
		"0b70b5bc-da3e-4a2b-9f11-3c2d1e0f9a8b", // uuid 风格
		strings.Repeat("a", 64),                // 长度上限恰好
	} {
		w := doProbe(t, newRequestIDRouter(), in, true)
		if got := w.Header().Get(consts.HeaderRequestID); got != in {
			t.Errorf("合法入站编号 %q: 响应头 = %q, want 原样沿用", in, got)
		}
		if body := w.Body.String(); body != in {
			t.Errorf("合法入站编号 %q: context = %q, want 原样沿用", in, body)
		}
	}
}

// 不规矩的入站编号（超长/控制字符/空白/非 ASCII/空值）一律弃用重生成——
// 服务端日志字段不许被外部输入污染。
func TestRequestID_RegeneratesInvalidInbound(t *testing.T) {
	invalids := []string{
		strings.Repeat("a", 65),
		"has space",
		"inject\r\nfake-log-line",
		"编号-中文",
		"",
	}
	for _, in := range invalids {
		w := doProbe(t, newRequestIDRouter(), in, true)
		rid := w.Header().Get(consts.HeaderRequestID)
		if rid == in && in != "" {
			t.Errorf("非法入站编号 %q 被原样沿用", in)
		}
		if rid == "" || rid == in {
			t.Errorf("非法入站编号 %q: 期望重生成, got %q", in, rid)
		}
		if len(rid) != 12 {
			t.Errorf("非法入站编号 %q: 重生成结果 %q 非 12 位", in, rid)
		}
	}
}

// 中间件必须同时把编号回写入站头：/v1 透传白名单从请求头取值，
// 访问日志 formatter 也读请求头——三处（日志/响应/上游）指向同一个号。
func TestRequestID_NormalizesRequestHeader(t *testing.T) {
	r := gin.New()
	var seen string
	r.Use(RequestID())
	r.GET("/probe", func(c *gin.Context) {
		seen = c.Request.Header.Get(consts.HeaderRequestID)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	// 生成场景：请求头被回填
	if seen == "" || len(seen) != 12 {
		t.Errorf("中间件未回写入站请求头, got %q", seen)
	}
}
