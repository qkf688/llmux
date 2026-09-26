package importexport

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
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

// TestImportConfig_PanicBubblesToRecovery 锁「导入路径 panic 必须可见」（Stage 25）。
// 历史缺陷：defer recover 只回滚不重抛不留痕——panic 被吞成空响应，事务回滚后 handler
// 继续往下走、对外像成功（outcome 零值），服务端零留痕。修后事务改 Transaction 闭包
// （panic 时回滚并上抛，GORM 框架契约），由 middleware.Recovery 收口为 500 + [req=]
// 编号、栈进日志。panic 注入：测试库注册 GORM Create 前置回调 panic（确定性注入点）；
// 中间件挂 RequestID + Recovery 对齐真实装配序。
func TestImportConfig_PanicBubblesToRecovery(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	db := newImportTestDB(t)
	if err := db.Callback().Create().Before("gorm:create").Register("test:panic", func(tx *gorm.DB) {
		panic("boom: injected via test callback")
	}); err != nil {
		t.Fatalf("register panic callback: %v", err)
	}
	oldDB := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = oldDB })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.POST("/import", ImportConfig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/import",
		strings.NewReader(`{"providers":[{"name":"a"}]}`)))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应上抛为 500, got %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	rid := w.Header().Get(consts.HeaderRequestID)
	if rid == "" || !strings.Contains(body, "[req="+rid+"]") {
		t.Errorf("panic 型 500 响应缺编号文案（rid=%q）: %s", rid, body)
	}
	line := strings.TrimSpace(logs.String())
	if !strings.Contains(line, "level=ERROR") || !strings.Contains(line, "stack") {
		t.Errorf("panic 留痕缺 ERROR 级或栈: %s", line)
	}
}

// TestImportConfig_PanicRollsBackAllWrites 配套锁回滚：panic 发生在 providers 段
// 已清表之后（clear 已删除预置行、首个 Create 即 panic）→ Transaction 闭包回滚，
// 预置数据必须原样存活——半残写入不落库。
func TestImportConfig_PanicRollsBackAllWrites(t *testing.T) {
	db := newImportTestDB(t)
	if err := db.Create(&models.Provider{Name: "keep", Type: "openai"}).Error; err != nil {
		t.Fatal(err)
	}
	// 注册回调后注册的 panic 只打一次（首个 Create 即触发），无需计数控制。
	if err := db.Callback().Create().Before("gorm:create").Register("test:panic", func(tx *gorm.DB) {
		panic("boom: injected via test callback")
	}); err != nil {
		t.Fatalf("register panic callback: %v", err)
	}
	oldDB := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = oldDB })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.POST("/import", ImportConfig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/import",
		strings.NewReader(`{"providers":[{"name":"new"},{"name":"new2"}]}`)))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应上抛为 500, got %d, body=%s", w.Code, w.Body.String())
	}
	var n int64
	db.Model(&models.Provider{}).Count(&n)
	if n != 1 {
		t.Fatalf("panic 后 providers 行数 = %d, want 1（事务回滚、清表不生效）", n)
	}
	var p models.Provider
	if err := db.First(&p).Error; err != nil {
		t.Fatal(err)
	}
	if p.Name != "keep" {
		t.Fatalf("存活行 = %q, want keep（原数据未被半残导入覆盖）", p.Name)
	}
}

// TestImportConfig_SuccessResponseShape 锁导入成功的响应体键契约。
//
// 前端数据库页按类型汇总展示「已导入: 提供商 N 条」，此前只回 `{message:"配置导入成功"}`
// 而前端仍按分类型计数读，导致**每次成功导入都弹「导入失败」**（读取 undefined 抛错被
// 自己的 catch 吞成失败文案）且弹窗不关。契约测试必须打在响应体 JSON 上（AGENTS §5）：
// 只断 handler 内部变量测不出键名/形状断层。
func TestImportConfig_SuccessResponseShape(t *testing.T) {
	db := newImportTestDB(t)
	oldDB := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = oldDB })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/import", ImportConfig)

	body := `{"providers":[{"name":"a"},{"name":"b"}],"model_providers":[{"id":1}],"settings":[{"key":"k"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d, body=%s", w.Code, w.Body.String())
	}
	if got := gjson.Get(w.Body.String(), "code").Int(); got != 200 {
		t.Fatalf("信封 code = %d, body = %s", got, w.Body.String())
	}
	// 各类计数键必须存在且值正确（templates 不在导入注册表，恒零值键也在）
	wants := map[string]int64{
		"data.providers.imported":    2,
		"data.providers.skipped":     0,
		"data.associations.imported": 1,
		"data.settings.imported":     0,
		"data.settings.skipped":      1,
		"data.templates.imported":    0,
	}
	for path, want := range wants {
		res := gjson.Get(w.Body.String(), path)
		if !res.Exists() {
			t.Fatalf("响应缺少键 %s，body = %s", path, w.Body.String())
		}
		if res.Int() != want {
			t.Fatalf("%s = %d, want %d", path, res.Int(), want)
		}
	}
}
