package importexport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/gjson"
)

// Stage 2 资源防御：导入接口复用入站硬上限 consts.MaxInboundBodyBytes。超限走管理端
// BadRequest 信封（HTTP 200 + 信封 code 400）而非真实 413——管理端信封惯例与同域导出
// 端点一致（/v1 的 413 是另一套协议形状）。边界用例锁 off-by-one：恰好等于上限必须
// 放行到解析段（JSON 解析失败 ≠ 体积超限）。
func TestImportConfig_RequestBodySizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/import", ImportConfig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/import",
		strings.NewReader(strings.Repeat("a", consts.MaxInboundBodyBytes+1))))
	if got := gjson.Get(w.Body.String(), "message").String(); !strings.Contains(got, "超过上限") {
		t.Fatalf("超限响应 = %q, want 含「超过上限」", got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("管理端应走信封（HTTP 200），got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/import",
		strings.NewReader(strings.Repeat("a", consts.MaxInboundBodyBytes))))
	msg := gjson.Get(w2.Body.String(), "message").String()
	if strings.Contains(msg, "超过上限") {
		t.Fatalf("恰等于上限被误判超限: %q", msg)
	}
	if !strings.Contains(msg, "解析") {
		t.Fatalf("恰等于上限应已放行到解析段, got %q", msg)
	}
}
