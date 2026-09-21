package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

func init() { gin.SetMode(gin.TestMode) }

// generatedRIDPattern 自生成编号的口径：`gw:` 前缀 + 12 位小写 hex（Stage 17 来源标记）。
var generatedRIDPattern = regexp.MustCompile(`^gw:[0-9a-f]{12}$`)

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
	if !generatedRIDPattern.MatchString(rid) {
		t.Errorf("生成的编号 = %q, want gw: 前缀 + 12 位 hex", rid)
	}
	if body := w.Body.String(); body != rid {
		t.Errorf("context 中的编号 = %q, 与响应头 %q 不一致", body, rid)
	}
}

// 调用方自带规矩的编号 → 沿用内核并打 in: 来源前缀（外部 trace 体系能对上服务端
// 日志，且与网关自生成号在同一个字段里可区分来源——Stage 17 防串号），
// 必须回显在响应头，用户报障时拿得到同一个号。
func TestRequestID_ReusesValidInbound(t *testing.T) {
	for _, in := range []string{
		"req_abc123",
		"0b70b5bc-da3e-4a2b-9f11-3c2d1e0f9a8b", // uuid 风格
		strings.Repeat("a", 64),                // 内核长度上限恰好
	} {
		want := consts.RequestIDPrefixInbound + in
		w := doProbe(t, newRequestIDRouter(), in, true)
		if got := w.Header().Get(consts.HeaderRequestID); got != want {
			t.Errorf("合法入站编号 %q: 响应头 = %q, want %q（沿用+in: 前缀）", in, got, want)
		}
		if body := w.Body.String(); body != want {
			t.Errorf("合法入站编号 %q: context = %q, want %q", in, body, want)
		}
	}
}

// 入站值自带 gw:/in: 来源前缀 → 判非法重生成，绝不能叠出 in:gw:… 双前缀
// （前缀是网关打的来源标记，外部不得自选；自选 gw: 号会污染自生成号空间）。
func TestRequestID_PrefixedInboundRegenerated(t *testing.T) {
	for _, in := range []string{
		consts.RequestIDPrefixGenerated + "0b70b5bcda3e",
		consts.RequestIDPrefixInbound + "req_abc123",
	} {
		w := doProbe(t, newRequestIDRouter(), in, true)
		rid := w.Header().Get(consts.HeaderRequestID)
		if !generatedRIDPattern.MatchString(rid) {
			t.Errorf("带前缀入站编号 %q: 期望重生成为 gw: 口径, got %q", in, rid)
		}
		if strings.HasSuffix(rid, in) {
			t.Errorf("带前缀入站编号 %q 被叠进生成值 %q（双前缀）", in, rid)
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
		if rid == "" || rid == in {
			t.Errorf("非法入站编号 %q: 期望重生成, got %q", in, rid)
		}
		if !generatedRIDPattern.MatchString(rid) {
			t.Errorf("非法入站编号 %q: 重生成结果 %q 非 gw: 口径", in, rid)
		}
	}
}

func TestRequestID_GeneratedIDsAreDistinct(t *testing.T) {
	// 48-bit 熵下单实例排障够用，但撞号=两条请求共享同一取证键；100 次抽样去重
	// 是成本最低的回归哨兵（真撞了这里先红）。
	r := newRequestIDRouter()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		rid := doProbe(t, r, "", false).Header().Get(consts.HeaderRequestID)
		if seen[rid] {
			t.Fatalf("第 %d 次生成撞号: %q", i, rid)
		}
		seen[rid] = true
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

	// 生成场景：请求头被回填 gw: 口径编号
	if !generatedRIDPattern.MatchString(seen) {
		t.Errorf("中间件未回写入站请求头（want gw: 口径）, got %q", seen)
	}
}
