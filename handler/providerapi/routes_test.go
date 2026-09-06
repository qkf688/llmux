package providerapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
)

// TestRegisterHead_InlineKeysRawRouteMatches 锁定受控明文端点的路由注册契约：
// 固定段 `inline-keys/raw` 与两个路径参数（`:id` / `:groupId`）在真实 engine 下必须
// 命中 GetGroupInlineKeys——手工 CreateTestContext + 手塞 c.Params 的既有测试测不出
// 参数名拼写与 gin 路由冲突（`:id` 与既有 `/providers/models/:id`、`/providers/template`
// 同层共存），这里走一次真实 ServeHTTP。
//
// 判别式：provider 9999 不存在 → handler 先做供应商存在性校验 → 信封 code=404
// "Provider not found"。若路由未命中，gin 返回自己的 404（body 无该文案）；
// 若 groupId 参数名与 handler 的 ParseUintParamAllowZero(c,"groupId") 不一致，
// 会先解析失败 → "Invalid ID format"。
func TestRegisterHead_InlineKeysRawRouteMatches(t *testing.T) {
	testsupport.InitTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHead(r)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/providers/9999/groups/8888/inline-keys/raw", nil))
	if w.Code != 200 {
		t.Fatalf("HTTP %d, 期望 200 信封（错误码在信封内）, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "Invalid ID format") {
		t.Fatalf("路径参数名与 handler 不一致（:id/:groupId 解析失败）: body=%s", body)
	}
	if !strings.Contains(body, "Provider not found") {
		t.Fatalf("未命中 GetGroupInlineKeys（路由未注册或被固定段劫持）: body=%s", body)
	}
}
