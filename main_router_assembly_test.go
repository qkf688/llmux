package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/handler"
	"github.com/qkf688/llmux/handler/testsupport"
)

// 本文件锁引擎装配序（framework-conventions §1）的两半：
//
//	TestGateRouterMiddlewareOrder —— 顺序本体（源码级机械门）；
//	TestRouterAssembly_RequestIDStamped —— 顺序的可观察后果（401/404/500 都有号）。
//
// 两半缺一不可：gin 的访问行 formatter 在 c.Next() 之后才读请求头，此刻后置的
// RequestID 也已回写完毕，行为上根本区分不出「谁先登记」；而源码门只认文本次序，
// 认不出「Use 挪去 RegisterAll 之后所以压根没生效」。判据参照
// main_startup_sanitize_gate_test.go 的装配位锁定先例。

// newRouterSourceBody 取出 main.go 里 newRouter 的函数体（到下一个顶层 func 为止），
// 让顺序门只看装配代码本身，不受 main.go 其余部分干扰。
func newRouterSourceBody(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读取 main.go: %v", err)
	}
	content := string(src)
	start := strings.Index(content, "func newRouter(")
	if start < 0 {
		t.Fatal("main.go 未找到 func newRouter(——装配函数被删除或改名，本门判据与 §1 需同步更新")
	}
	body := content[start:]
	if end := strings.Index(body, "\nfunc "); end > 0 {
		body = body[:end]
	}
	return body
}

// TestGateRouterMiddlewareOrder 锁定四个中间件的装配次序与挂接位置：
//
//   - 次序：RequestID → AccessLog → Recovery → gzip（RequestID 挪后则访问行读不到号，
//     Recovery 挪前则 panic 留痕取不到号，两者都是 §1 明文禁止的重排）；
//   - 位置：全部 router.Use 必须先于 handler.RegisterAll——gin 为已注册路由快照
//     handler 链，Use 写在后面那些路由根本不带它，是最隐蔽的一种「装配没生效」；
//   - 数量：中间件增删必须同步本门与 §1（新增一个 Use 也属于装配契约变更）。
func TestGateRouterMiddlewareOrder(t *testing.T) {
	body := newRouterSourceBody(t)

	markers := []struct {
		want string
		why  string
	}{
		{"router.Use(middleware.RequestID())", "RequestID 最先：401/404/panic 也要有号可对"},
		{"router.Use(middleware.AccessLog())", "AccessLog 读 RequestID 回写的入站头快照，必须在其后"},
		{"router.Use(middleware.Recovery())", "Recovery 的 panic 留痕取 rid，须晚于 RequestID"},
		{"router.Use(gzip.Gzip(", "gzip 最后（SSE 由 WithExcludedPaths 排除 /v1/）"},
	}

	prev := -1
	for _, m := range markers {
		n := strings.Count(body, m.want)
		if n != 1 {
			t.Fatalf("装配位 %q 在 newRouter 内出现 %d 次，want 1（%s）", m.want, n, m.why)
		}
		at := strings.Index(body, m.want)
		if at < prev {
			t.Fatalf("装配序被重排：%q 位于 offset %d，早于前一位（offset %d）——%s", m.want, at, prev, m.why)
		}
		prev = at
	}

	if n := strings.Count(body, "router.Use("); n != len(markers) {
		t.Fatalf("newRouter 内 router.Use 共 %d 处，want %d——中间件增删属装配契约变更，须同步本门与 framework-conventions §1", n, len(markers))
	}

	reg := strings.Index(body, "handler.RegisterAll(router")
	if reg < 0 {
		t.Fatal("newRouter 内未找到 handler.RegisterAll(router ……：路由注册被挪走或改写，本门判据需同步更新")
	}
	if lastUse := strings.LastIndex(body, "router.Use("); lastUse > reg {
		t.Fatalf("router.Use（offset %d）写在 handler.RegisterAll（offset %d）之后：gin 为已注册路由快照 handler 链，后置的 Use 对那些路由不生效", lastUse, reg)
	}
}

// requestIDValueRe 请求编号取值形状：网关自产 gw: / 沿用入站 in: + 内核（Stage 17）。
var requestIDValueRe = regexp.MustCompile(`^(gw|in):[A-Za-z0-9_\-]{1,64}$`)

// TestRouterAssembly_RequestIDStamped 锁装配序的**后果**：三条失败路径的响应都要带
// 请求编号——401（组内鉴权拦下，走不到路由）、404（NoRoute，根本无路由）、
// 500（panic 走 httpresp 收口出口），外加访问行与 panic 留痕指向同一号。
// RequestID 被挪出全局链、或挪到 RegisterAll 之后，这里当场红。
func TestRouterAssembly_RequestIDStamped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)

	logs := captureSlogTo(t)
	// gin.DefaultWriter 在 AccessLog() 创建时被捕获，必须先接管再装配引擎。
	access := captureAccessLogTo(t)

	r, err := newRouter(handler.Deps{JWTSecret: "assembly-test-secret"}, parseTrustedProxies(""))
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	// panic 探针必须在 newRouter **之后**注册：gin 为路由快照 handler链，
	// 后注册才带得上刚装配的全局中间件链（也就是真正走 Recovery）。
	r.GET("/__assembly/panic", func(*gin.Context) { panic("assembly test panic") })

	const panicText = "assembly test panic"

	cases := []struct {
		name       string
		path       string
		inboundRID string // 入站自带编号（验 in: 沿用），空则自生成
		wantStatus int
		wantPanic  bool
	}{
		{"401 组内鉴权拦截", "/api/logs", "", http.StatusUnauthorized, false},
		{"401 沿用入站编号打 in: 前缀", "/api/logs", "my-trace-123", http.StatusUnauthorized, false},
		{"404 NoRoute", "/api/__no_such_route__", "", http.StatusNotFound, false},
		{"500 panic 走 httpresp 收口", "/__assembly/panic", "", http.StatusInternalServerError, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accessBefore, logsBefore := access.Len(), logs.Len()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.inboundRID != "" {
				req.Header.Set("X-Request-Id", tc.inboundRID)
			}
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP %d, want %d（body=%s）", w.Code, tc.wantStatus, w.Body.String())
			}

			rid := w.Header().Get("X-Request-Id")
			if !requestIDValueRe.MatchString(rid) {
				t.Fatalf("X-Request-Id = %q, want gw:/in: 前缀编号（响应无号则用户报障时对不上日志）", rid)
			}
			if tc.inboundRID != "" && rid != "in:"+tc.inboundRID {
				t.Fatalf("沿用入站编号 = %q, want %q（Stage 17 来源前缀口径）", rid, "in:"+tc.inboundRID)
			}

			accessNew := access.String()[accessBefore:]
			if !strings.Contains(accessNew, "rid="+rid) {
				t.Fatalf("访问行缺 rid=%s（AccessLog 未挂上全局链，或排在 RequestID 之前）:\n%s", rid, accessNew)
			}

			logsNew := logs.String()[logsBefore:]
			if tc.wantPanic {
				body := w.Body.String()
				want := "internal server error [req=" + rid + "]"
				if !strings.Contains(body, want) {
					t.Fatalf("panic 响应未走 httpresp 收口出口，body=%s", body)
				}
				if strings.Contains(body, panicText) {
					t.Fatalf("panic 原文外泄到响应体: %s", body)
				}
				if !strings.Contains(logsNew, "request_id="+rid) {
					t.Fatalf("panic 留痕缺 request_id=%s（Recovery 未挂上全局链，或排在 RequestID 之前）:\n%s", rid, logsNew)
				}
				if !strings.Contains(logsNew, panicText) {
					t.Fatalf("panic 留痕缺 panic 原文:\n%s", logsNew)
				}
			}
		})
	}
}

// captureSlogTo 把 slog 默认输出重定向到 buffer（与 httpresp 包同名 helper 同构，
// 各自包内私有：共享它要跨包导出，反而把测试夹具变成公共契约）。
func captureSlogTo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// captureAccessLogTo 接管 gin 访问行的输出（gin.LoggerWithConfig 的默认 Writer）。
func captureAccessLogTo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := gin.DefaultWriter
	gin.DefaultWriter = &buf
	t.Cleanup(func() { gin.DefaultWriter = prev })
	return &buf
}
