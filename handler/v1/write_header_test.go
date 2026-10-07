package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

func init() { gin.SetMode(gin.TestMode) }

// Stage 3：响应头 X-Request-Id 唯一归属网关。上游（Azure/OpenAI 常见）自带同名头时
// 必须改名 X-Upstream-Request-Id 回传——否则客户端在一个头里看到 [网关号, 供应商号]
// 两值，取哪个全凭 SDK 实现，报障编号失去意义；改名后向供应商报障仍拿得到原号。
// Stage 5 的隐含兼容前提：网关**从不**把上游状态码透传给调用方。上游回 201 的兼容网关
// 现在被判成功（以前判失败），若哪天有人把上游状态码搬到响应上，严格判 `status == 200`
// 的 SDK 会全线报错——而这条前提此前没有任何测试锁定。成功路径的状态码由 gin 首次写出
// 时定为 200，writeHeader 只搬头不写码，这里把「只搬头」钉住。
func TestWriteHeader_NeverEmitsUpstreamStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	up := http.Header{}
	up.Set("Content-Type", "application/json")

	writeHeader(c, false, up)
	_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("客户端状态码 = %d, want 200（上游 201 不得透传，否则严格判 200 的 SDK 全炸）", got)
	}
}

func TestWriteHeader_UpstreamRequestIDRenamedNotAppended(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	up := http.Header{}
	up.Set(consts.HeaderRequestID, "vendor-xyz")
	up.Set("Content-Type", "application/json")

	writeHeader(c, false, up)

	if got := w.Header().Values(consts.HeaderRequestID); len(got) != 0 {
		t.Errorf("网关命名空间被上游值占用: %v", got)
	}
	if got := w.Header().Get("X-Upstream-Request-Id"); got != "vendor-xyz" {
		t.Errorf("上游编号应改名为 X-Upstream-Request-Id, got %q", got)
	}
}

// TestWriteHeader_OnlyWhitelistedUpstreamHeaders 锁响应头允许清单（AC-4）：
// 网关只把「客户端真的用得上的头」发出去——协议形状（Content-Type）、有实际用途的
// 节奏信息（多久后重试、还剩多少额度）；其余（上游内部标记、长度、编码）一律不发。
//
// 为什么不能裸拷：上游回的每个头都会进调用方看到的响应，供应商内部的节点名、内部
// 调试号、外部不认识的追踪头全在里面；供应商换一家或改实现，这些头跟着变，调用方
// 却可能已经依赖上了。列表也一样要拦 Content-Length / Content-Encoding——网关不做
// 压缩转发（发往上游的 Accept-Encoding 已被剥掉、由传输层透明解压），长度交给
// net/http 自己算；照抄上游值在正文被改写时会截断响应。
func TestWriteHeader_OnlyWhitelistedUpstreamHeaders(t *testing.T) {
	cases := []struct {
		name          string
		upstreamKey   string
		upstreamValue string
		wantKept      bool
	}{
		{name: "协议形状头保留", upstreamKey: "Content-Type", upstreamValue: "application/json", wantKept: true},
		{name: "多久后重试保留", upstreamKey: "Retry-After", upstreamValue: "60", wantKept: true},
		{name: "限流剩余额度保留", upstreamKey: "x-ratelimit-remaining-requests", upstreamValue: "17", wantKept: true},
		{name: "供应商限流族保留", upstreamKey: "anthropic-ratelimit-requests-remaining", upstreamValue: "42", wantKept: true},
		{name: "响应体长度剔除", upstreamKey: "Content-Length", upstreamValue: "9999", wantKept: false},
		{name: "内容编码剔除", upstreamKey: "Content-Encoding", upstreamValue: "gzip", wantKept: false},
		{name: "供应商内部节点名剔除", upstreamKey: "X-Internal-Node", upstreamValue: "hz-03", wantKept: false},
		{name: "供应商内部调试号剔除", upstreamKey: "X-Upstream-Debug-Trace", upstreamValue: "abc", wantKept: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			up := http.Header{}
			up.Set(tc.upstreamKey, tc.upstreamValue)

			writeHeader(c, false, up)

			got := w.Header().Values(http.CanonicalHeaderKey(tc.upstreamKey))
			if tc.wantKept && len(got) != 1 {
				t.Errorf("%s = %v, want 保留一个值 %q", tc.upstreamKey, got, tc.upstreamValue)
			}
			if !tc.wantKept && len(got) != 0 {
				t.Errorf("%s = %v, want 不发（上游头不在允许清单里）", tc.upstreamKey, got)
			}
		})
	}
}

// 流式响应的分帧头由网关自己写（不由上游决定）：允许清单不得伤到它们，
// 同时上游的未知头仍不进。
func TestWriteHeader_StreamKeepsGatewayFramingHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	up := http.Header{}
	up.Set("X-Internal-Node", "hz-03")
	up.Set("Content-Type", "text/event-stream")

	writeHeader(c, true, up)

	for _, key := range []string{"Content-Type", "Cache-Control", "Connection", "X-Accel-Buffering"} {
		if got := w.Header().Get(key); got == "" {
			t.Errorf("%s 缺失，want 流式分帧头恒在", key)
		}
	}
	if got := w.Header().Values("X-Internal-Node"); len(got) != 0 {
		t.Errorf("X-Internal-Node = %v, want 不发（流式同样只发允许清单内的上游头）", got)
	}
}
