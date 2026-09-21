package middleware

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
)

// TestRequestID_ConsistencyAcrossConsumers 锁「三处格式同步」契约（Stage 17 AC-2/AC-3）：
// 一次请求的响应头 X-Request-Id、httpresp 500 文案 [req=]、slog 错误行 request_id 字段、
// 访问行 rid= 必须是同一个带来源前缀的编号（自生成 gw: / 沿用 in:）。
// 三处消费点全部走「值透传」（context/请求头），任何一处自行拼前缀或丢前缀即红。
func TestRequestID_ConsistencyAcrossConsumers(t *testing.T) {
	var accessBuf bytes.Buffer
	prevWriter := gin.DefaultWriter
	gin.DefaultWriter = &accessBuf
	defer func() { gin.DefaultWriter = prevWriter }()

	var slogBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&slogBuf, nil)))
	defer slog.SetDefault(prevLogger)

	r := gin.New()
	r.Use(RequestID(), AccessLog())
	r.GET("/boom", func(c *gin.Context) {
		httpresp.InternalServerError(c, errors.New("contract probe"))
	})

	cases := []struct {
		name       string
		inbound    string
		wantExact  string // 沿用场景：整值确定
		wantPrefix string // 自生成场景：只锁前缀
	}{
		{name: "自生成号带gw前缀", wantPrefix: consts.RequestIDPrefixGenerated},
		{name: "沿用号带in前缀", inbound: "trace-0921", wantExact: consts.RequestIDPrefixInbound + "trace-0921"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accessBuf.Reset()
			slogBuf.Reset()

			req := httptest.NewRequest(http.MethodGet, "/boom", nil)
			if tc.inbound != "" {
				req.Header.Set(consts.HeaderRequestID, tc.inbound)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			rid := w.Header().Get(consts.HeaderRequestID)
			if rid == "" {
				t.Fatal("响应头缺 X-Request-Id")
			}
			if tc.wantExact != "" && rid != tc.wantExact {
				t.Fatalf("响应头编号 = %q, want %q", rid, tc.wantExact)
			}
			if tc.wantPrefix != "" && !strings.HasPrefix(rid, tc.wantPrefix) {
				t.Fatalf("响应头编号 = %q, want 前缀 %q", rid, tc.wantPrefix)
			}

			// httpresp 500 文案（AC-2）
			if body := w.Body.String(); !strings.Contains(body, "[req="+rid+"]") {
				t.Errorf("500 文案 = %q, want 含 [req=%s]", body, rid)
			}
			// slog 错误行 request_id 字段（AC-3；TextHandler 对含冒号值可能加引号，两种形态都认）
			if s := slogBuf.String(); !strings.Contains(s, "request_id="+rid) &&
				!strings.Contains(s, "request_id=\""+rid+"\"") {
				t.Errorf("slog 输出 = %q, want 含 request_id=%s", s, rid)
			}
			// 访问行 rid=（AC-3）
			if line := accessBuf.String(); !strings.Contains(line, "rid="+rid) {
				t.Errorf("访问行 = %q, want 含 rid=%s", line, rid)
			}
		})
	}
}
