package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler"
	"github.com/qkf688/llmux/handler/testsupport"
)

// 本文件锁引擎的信任边界（Stage 20）：ClientIP 是访问日志/归因的事实字段，
// 若采信任意来源的 X-Forwarded-For，它就是外部可控值，溯源结论不可信。
// 契约：TRUSTED_PROXIES 显式配置可信代理网段，未设/空 = 全关（直连默认安全）；
// 注意 gin 默认（从不调用 SetTrustedProxies）= 信任全部代理，必须显式调用才收窄。

// clientIPProbe 在引擎上挂一个回显 ClientIP() 的探针路由，直接观察信任边界的判定结果。
func clientIPProbe(t *testing.T, r *gin.Engine) {
	t.Helper()
	r.GET("/__probe/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, "%s", c.ClientIP())
	})
}

// probeClientIP 以指定 TCP 对端 + X-Forwarded-For 打引擎，返回引擎判定的 ClientIP。
func probeClientIP(t *testing.T, r *gin.Engine, remoteAddr, forwardedFor string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__probe/client-ip", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("探针 HTTP %d, want 200（body=%s）", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestRouterTrustedProxies_IgnoreForgedXFF 钉默认安全：未配置可信代理时，
// 直连请求伪造 X-Forwarded-For 不得改变 ClientIP（否则访问行 IP 是外部可控值）。
// 此前 gin 默认信任全部代理，本用例在落地前实测红。
func TestRouterTrustedProxies_IgnoreForgedXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)
	_ = captureAccessLogTo(t)

	r, err := newRouter(handler.Deps{JWTSecret: "trusted-proxies-test"}, parseTrustedProxies(""))
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	clientIPProbe(t, r)

	got := probeClientIP(t, r, "192.0.2.1:1234", "203.0.113.9")
	if got != "192.0.2.1" {
		t.Fatalf("ClientIP = %q, want %q（TCP 对端；伪造 XFF 被采信 = 访问日志 IP 可伪造）", got, "192.0.2.1")
	}
}

// TestRouterTrustedProxies_NilMeansClosed 钉 newRouter 的 nil 输入契约：nil 与空切片
// 在 gin v1.10 同为全关（真正「信任全部」的是从不调用 SetTrustedProxies），本用例
// 防回归到「漏调 SetTrustedProxies」的默认放行状态。
func TestRouterTrustedProxies_NilMeansClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)
	_ = captureAccessLogTo(t)

	r, err := newRouter(handler.Deps{JWTSecret: "trusted-proxies-test"}, nil)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	clientIPProbe(t, r)

	got := probeClientIP(t, r, "192.0.2.1:1234", "203.0.113.9")
	if got != "192.0.2.1" {
		t.Fatalf("ClientIP = %q, want %q（nil 输入后伪造 XFF 不得生效——newRouter 的 nil 契约）", got, "192.0.2.1")
	}
}

// TestRouterTrustedProxies_HonorConfiguredProxy 钉配置生效的另一半：显式配置的
// 网段内代理所转的 XFF 要采信；网段外的伪造仍无效（信任边界按网段逐跳判定）。
func TestRouterTrustedProxies_HonorConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)
	_ = captureAccessLogTo(t)

	r, err := newRouter(handler.Deps{JWTSecret: "trusted-proxies-test"}, []string{"192.0.2.0/24"})
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	clientIPProbe(t, r)

	if got, want := probeClientIP(t, r, "192.0.2.1:1234", "203.0.113.9"), "203.0.113.9"; got != want {
		t.Fatalf("可信代理转发的 ClientIP = %q, want %q（配置的网段内代理应采信其 XFF）", got, want)
	}
	if got, want := probeClientIP(t, r, "198.51.100.7:1234", "203.0.113.9"), "198.51.100.7"; got != want {
		t.Fatalf("网段外伪造的 ClientIP = %q, want %q（非可信来源的 XFF 不得采信）", got, want)
	}
}

// TestRouterTrustedProxies_RejectInvalidValue 钉 fail fast（AC-3）：非法信任边界
// 配置必须让装配失败并带上原值，静默回退（无论全开还是全关）都会掩盖配置错误。
func TestRouterTrustedProxies_RejectInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)

	_, err := newRouter(handler.Deps{JWTSecret: "trusted-proxies-test"}, []string{"not-a-proxy"})
	if err == nil {
		t.Fatal("newRouter 对非法 TRUSTED_PROXIES 值返回 nil error，want 报错（信任边界配置错误必须 fail fast）")
	}
	if !strings.Contains(err.Error(), "not-a-proxy") {
		t.Fatalf("错误信息 %q 应带原值（fail fast 要能定位是哪项配置错了）", err.Error())
	}
}

// TestParseTrustedProxies 钉解析口径：逗号分隔 + 去空白 + 丢空项；空输入恒返回
// 非 nil 空切片（nil 是 gin 的「信任全部」，绝不能从解析层漏出去）。
func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"空串", "", []string{}},
		{"纯空白", "   ", []string{}},
		{"只有分隔符", ",,,", []string{}},
		{"单个 CIDR", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"单个 IP", "127.0.0.1", []string{"127.0.0.1"}},
		{"多值带空白", " 1.1.1.1 , 2.2.2.0/24 ", []string{"1.1.1.1", "2.2.2.0/24"}},
		{"中间空项", "1.1.1.1,,2.2.2.0/24", []string{"1.1.1.1", "2.2.2.0/24"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTrustedProxies(tc.raw)
			if got == nil {
				t.Fatalf("parseTrustedProxies(%q) = nil, want 非 nil（nil 在 gin 语义里是信任全部代理）", tc.raw)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseTrustedProxies(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseTrustedProxies(%q) = %v, want %v", tc.raw, got, tc.want)
				}
			}
		})
	}
}
