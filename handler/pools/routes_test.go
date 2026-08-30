package pools

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
)

// TestRegisterRouteOrder_BatchBeforeCredID 锁定注册顺序契约：
// /pools/:id/credentials/batch/* 固定段必须优先于 /pools/:id/credentials/:credId 注册，
// 否则 gin 会把 "batch" 当成 credId 捕获，批量端点被单条路径劫持。
//
// 判别式：batch 固定段若正确命中批量端点，会先做号池存在性校验，
// 池 9999 不存在 → code=404 "Pool not found"；
// 若被 :credId 劫持，则会先解析 credId="batch" → code=400 "Invalid ID format"。
func TestRegisterRouteOrder_BatchBeforeCredID(t *testing.T) {
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
		{"batch/status 固定段优先", "PATCH", "/pools/9999/credentials/batch/status", `{"ids":[1],"status":"disabled"}`},
		{"batch 固定段优先", "DELETE", "/pools/9999/credentials/batch", `{"ids":[1]}`},
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
			body := w.Body.String()
			if strings.Contains(body, "Pool not found") {
				return // 命中批量端点（池存在性 404）
			}
			if strings.Contains(body, "Invalid ID format") {
				t.Fatalf("batch 被 :credId 劫持（Invalid ID format）: body=%s", body)
			}
			t.Fatalf("未命中任何预期分支: body=%s", body)
		})
	}
}
