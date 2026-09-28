package pools

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
)

// 号池域的路由顺序契约：批量导入（静态段 batch）与按凭据测试（:credId 子树）必须共存且
// 互不遮蔽。凭据自身的批量端点已迁到 handler/credentials（那里的顺序契约由
// TestRegisterRouteOrder_StaticSegmentsBeforeID 锁定）。
//
// 判别式：两条路径都先做号池存在性校验，池 9999 不存在 → code=404 "Pool not found"；
// 被误吞/遮蔽时表现为 405/404 未注册或落进另一 handler 的解析错误。
func TestRegisterRouteOrder_ImportNotShadowedByCredIDTest(t *testing.T) {
	testsupport.InitTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"batch/import 未被 :credId/test 遮蔽", "POST", "/pools/9999/credentials/batch/import", `{"keys":["sk-route-import-ABCD1234"]}`},
		{"credId/test 正确路由", "POST", "/pools/9999/credentials/5/test", `{}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatalf("HTTP %d, 期望 200 信封, body=%s", w.Code, w.Body.String())
			}
			if body := w.Body.String(); !strings.Contains(body, "Pool not found") {
				t.Fatalf("未路由到预期端点（应命中池存在性 404）: body=%s", body)
			}
		})
	}
}
