package testapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
)

// Stage 15 契约：ReAct 诊断（openai 系走 SDK 直发）的凭据头方言必须与真实转发同源——
// 供应商 config 配 openai_auth_type=x-api-key 时，诊断出站头为 x-api-key=<cred> 且不带
// Authorization。方言判断单源在 providers（applyAuthHeader / OpenAICompatAuthHeaders），
// 诊断测试侧只搬运头。此前 SDK 路径恒发 bearer，x-api-key 方言中转在能力测试里必假阴性
// （真实转发可用、测试报失败 =「测绿≠能跑」的反向分叉）。

// TestTestReactHandler_XAPIKeyDialectUsesXAPIKeyHeader 假上游只认 x-api-key 方言
// （复刻「只认该头的中转」）：配 x-api-key 的 openai 型供应商 ReAct 诊断必须以
// x-api-key=<cred> 出站、不得携带 Authorization，否则上游 401 → 诊断假阴性。
func TestTestReactHandler_XAPIKeyDialectUsesXAPIKeyHeader(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	// 锁定「SDK 环境变量默认 key 被选项顺序清掉」这一隐式依赖（code-review S3）：
	// openai-go 默认选项先于用户选项应用，OPENAI_API_KEY 会注入 Authorization；
	// 若 SDK 升级改了装配序，下方「Authorization 键必须不存在」断言即红。
	t.Setenv("OPENAI_API_KEY", "env-leak-should-not-appear")

	var hits atomic.Int64
	var gotAuth, gotXKey atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotXKey.Store(r.Header.Get("x-api-key"))
		// 方言判据：x-api-key 正确且 Authorization 不存在才放行；
		// bearer 方言（Authorization: Bearer …）一律 401。
		if r.Header.Get("x-api-key") != "sk-xapi" || r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid x-api-key dialect"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"id":"chunk-1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`+"\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	provider := seedProviderRowWithConfig(t, "p-xapi", "openai",
		`{"base_url":"https://should-not-use.example","openai_auth_type":"x-api-key"}`)
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	seedInlineGroup(t, cipher, provider.ID, "xapi 组", "", "sk-xapi")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callReactTest(t, mwp.ID)

	if hits.Load() == 0 {
		t.Fatalf("react 测试未打到上游（装配阶段就失败）；SSE=%s", w.Body.String())
	}
	if got, _ := gotXKey.Load().(string); got != "sk-xapi" {
		t.Fatalf("上游 x-api-key 头 = %q, want sk-xapi（方言未贯通即本缺陷）", got)
	}
	if got, _ := gotAuth.Load().(string); got != "" {
		t.Fatalf("上游 Authorization 头 = %q, want 键不存在（x-api-key 方言不得并送 bearer）", got)
	}
	if body := w.Body.String(); strings.Contains(body, "invalid x-api-key dialect") {
		t.Fatalf("上游以方言拒绝了诊断请求（诊断假阴性 = 本缺陷现象）；SSE=%s", body)
	}
}

// TestTestReactHandler_EmptyCredentialFailsLocallyWithoutUpstream 凭据明文为空 =
// 配置错误而非供应商故障：诊断必须本地直接报「未配置 API Key」，零上游请求
// （不带着空凭据头出去挨 401，把配置问题伪装成上游故障）。
func TestTestReactHandler_EmptyCredentialFailsLocallyWithoutUpstream(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
	}))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-empty-key", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	seedInlineGroup(t, cipher, provider.ID, "空 key 组", "", "")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callReactTest(t, mwp.ID)

	if hits.Load() != 0 {
		t.Fatalf("上游 hits = %d, want 0（空密钥必须本地失败，不得发上游）", hits.Load())
	}
	body := w.Body.String()
	if !strings.Contains(body, "event:error") {
		t.Fatalf("SSE 未回 error 事件；body=%s", body)
	}
	if !strings.Contains(body, "未配置 API Key") {
		t.Fatalf("error 文案未含「未配置 API Key」（配置错误需与上游故障可区分）；body=%s", body)
	}
	if strings.Contains(body, "event:success") {
		t.Fatalf("空密钥不应报 success；body=%s", body)
	}
}
