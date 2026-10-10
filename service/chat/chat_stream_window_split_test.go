package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// 本文件是 Stage 38「单次尝试窗口按入站 stream 分流」的回归组：非流式请求上游要生成
// 完整回答才发响应头，故它的窗口语义是「整段生成预算」，与流式等头不同源。
// 断言一律落在可观测行为上：窗口内返回 = 成功；超过所属窗口 = 失败（对外 502 口径不变）。
// 设置桩复用 chat_balance_timeout_separation_test.go 的参数化 timeoutSettingsReader
// ——两个窗口在这里取**可区分**的值：两者同值时，分不出「按 stream 分流」与「沿用旧单一窗口」。

// TestResolveUpstreamWindow_SplitsByStream 锁分流判据本身：同一份设置下，两种入站形态
// 取到不同窗口，且**类别与值同源**（类别决定失败文案，值决定实际施加的窗口——两者若
// 各自分支，下次加入第三种入站形态时会静默印错类别）。
// 变异：resolver 忽略 stream 参数、恒取响应头键 → 两条断言都红。
func TestResolveUpstreamWindow_SplitsByStream(t *testing.T) {
	installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 30, nonStreamWindowSec: 90, maxRetry: 1})

	stream := resolveUpstreamWindow(context.Background(), true)
	if stream.Kind != upstreamWindowStream || stream.Window != 30*time.Second {
		t.Fatalf("流式窗口 = %+v, want 类别 stream / 30s（等上游响应头）", stream)
	}
	nonStream := resolveUpstreamWindow(context.Background(), false)
	if nonStream.Kind != upstreamWindowNonStream || nonStream.Window != 90*time.Second {
		t.Fatalf("非流式窗口 = %+v, want 类别 nonStream / 90s（等整段回答）", nonStream)
	}
}

// setupWindowSplitSlowProvider 建一个「延迟 delay 后才返回响应」的上游 + 最小可用路由
// （模型/供应商/端点/分组/凭据/关联），返回候选池与真实模型行。延迟发生在响应头之前——
// 非流式的「整段生成」与流式的「等头」在链路上就是同一段等待。
// 按请求体是不是流式返回对应形态（流式回合法 SSE、非流式回完整 JSON），使「窗口够长」
// 与「窗口太短」在两种入站形态下都产生**可区分的成功/失败**（否则窗口被整体放宽时
// 流式用例会因「SSE 收到 JSON」这类无关原因照样失败，断言不咬）。
func setupWindowSplitSlowProvider(t *testing.T, delay time.Duration) (CandidatePool, models.Model) {
	t.Helper()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		time.Sleep(delay)
		if strings.Contains(string(body), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(gateGoodSSE))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(slow.Close)

	model := models.Model{Name: "m-window-split"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "slow-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, slow.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	addLegacyChannelRows(t, cipher, provider)
	enabled := true
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "slow-upstream", Status: &enabled, Weight: 10, Priority: 10}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	return pool, model
}

// balanceOnce 用最小入站形态跑一次真实路径转发，只区分 stream 与否。
// 入站原文带 stream 标记（同真实请求）：上游请求体的流式与否由它决定，
// 上游据此选返回 SSE 还是完整 JSON，两侧形态才对得上。
func balanceOnce(t *testing.T, pool CandidatePool, modelName string, stream bool) (*BalanceResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	raw := fmt.Sprintf(`{"model":%q,"stream":%t,"messages":[{"role":"user","content":"hi"}]}`, modelName, stream)
	return BalanceChat(ctx, BalanceInput{
		Start:  time.Now(),
		Style:  "openai",
		Before: Before{Model: modelName, Stream: stream, raw: []byte(raw)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: modelName},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
}

// TestBalanceChat_NonStream_SlowUpstreamInsideNonStreamWindow_Succeeds 非流式请求的
// 现场形态：上游 1.6s 才发响应头（= 花 1.6s 生成完整回答）、非流式窗口 3s → 应成功。
// 变异：非流式也走流式的 1s 等头窗口（改动前的单一窗口语义）→ 本条红。
func TestBalanceChat_NonStream_SlowUpstreamInsideNonStreamWindow_Succeeds(t *testing.T) {
	pool, model := setupWindowSplitSlowProvider(t, 1600*time.Millisecond)
	installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 3, maxRetry: 1})

	result, err := balanceOnce(t, pool, model.Name, false)
	if err != nil {
		t.Fatalf("BalanceChat err = %v, want success（1.6s 在非流式 3s 窗口内；沿用流式 1s 窗口即红）", err)
	}
	if result.ProviderName != "slow-provider" {
		t.Fatalf("ProviderName = %q, want slow-provider", result.ProviderName)
	}
}

// TestBalanceChat_Stream_SlowUpstreamBeyondStreamWindow_StillFails 流式侧不被放宽：
// 同一个 1.6s 上游、流式等头窗口 1s → 仍判失败（分流不是「把窗口整体调大」）。
// 对外仍是 502「上游全挂」口径（Stage 38 拍板不改调用方可见状态码）。
func TestBalanceChat_Stream_SlowUpstreamBeyondStreamWindow_StillFails(t *testing.T) {
	pool, model := setupWindowSplitSlowProvider(t, 1600*time.Millisecond)
	installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 3, maxRetry: 1})

	_, err := balanceOnce(t, pool, model.Name, true)
	if err == nil {
		t.Fatal("err = nil, want 等头窗口超时失败（流式窗口未被放宽）")
	}
	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusBadGateway {
		t.Fatalf("err = %v, want 502（换家换不到仍是「上游全挂」口径）", err)
	}
}

// windowFailureLogError 读本次尝试落库的失败文本——面板「错误」列看到的就是它。
func windowFailureLogError(t *testing.T) string {
	t.Helper()
	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "slow-provider").First(&got).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("chat log status = %q, want error", got.Status)
	}
	return got.Error
}

// TestBalanceChat_WindowTimeout_FailureTextCarriesCategoryAndWindow 窗口掐断的失败文本
// 要能让面板分清「窗口掐的」与「上游自己断的」，并带出窗口秒数（Stage 38）：
// 流式与非流式两个分支各一条直达用例（各自窗口取 1s、另键留 3s，秒数可区分两个分支）。
// 变异：describeUpstreamWindowTimeout 直接返回原 err → 两条都红；两个分支文案写反 → 红。
func TestBalanceChat_WindowTimeout_FailureTextCarriesCategoryAndWindow(t *testing.T) {
	t.Run("流式等头窗口", func(t *testing.T) {
		pool, model := setupWindowSplitSlowProvider(t, 1600*time.Millisecond)
		installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 3, maxRetry: 1})

		if _, err := balanceOnce(t, pool, model.Name, true); err == nil {
			t.Fatal("err = nil, want 窗口超时失败")
		}
		got := windowFailureLogError(t)
		assertContains(t, got, "流式等头窗口")
		assertContains(t, got, "1 秒")
		assertContains(t, got, headerWindowTimeoutMarker) // 上游原文保留在同一条里
	})

	t.Run("非流式整段预算窗口", func(t *testing.T) {
		pool, model := setupWindowSplitSlowProvider(t, 1600*time.Millisecond)
		installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 3, nonStreamWindowSec: 1, maxRetry: 1})

		if _, err := balanceOnce(t, pool, model.Name, false); err == nil {
			t.Fatal("err = nil, want 窗口超时失败")
		}
		got := windowFailureLogError(t)
		assertContains(t, got, "非流式整段预算窗口")
		assertContains(t, got, "1 秒")
		assertContains(t, got, headerWindowTimeoutMarker)
	})
}

func assertContains(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("失败文本缺 %q，got %q", want, text)
	}
}

// windowTimeoutStubError 模拟 Do 阶段的超时错误形态：实现 net.Error 的 Timeout()，
// 文本可控（用于喂 net/http 的等头超时原文与其它来源的超时原文）。
type windowTimeoutStubError struct{ msg string }

func (e windowTimeoutStubError) Error() string { return e.msg }
func (windowTimeoutStubError) Timeout() bool   { return true }
func (windowTimeoutStubError) Temporary() bool { return false }

// TestDescribeUpstreamWindowTimeout_OnlyOurWindow 锁归因判据的边界：只有「我们的窗口 +
// net/http 等响应头超时原文」才贴类别；上游自己断连、连接/握手调优超时、请求 ctx 已到点
// （网关总超时族）、窗口未知四种情况一律原样返回——贴错类别比不贴更有害。
func TestDescribeUpstreamWindowTimeout_OnlyOurWindow(t *testing.T) {
	headerTimeout := windowTimeoutStubError{msg: `Post "http://up/v1/chat/completions": net/http: timeout awaiting response headers`}
	dialTimeout := windowTimeoutStubError{msg: "dial tcp 127.0.0.1:8800: i/o timeout"}
	upstreamReset := errors.New("read tcp 127.0.0.1:8800: connection reset by peer")

	expiredCtx, cancelExpired := context.WithCancel(context.Background())
	cancelExpired()

	tests := []struct {
		name      string
		ctx       context.Context
		err       error
		window    upstreamWindow
		wantParts []string // nil = 期望原样返回
	}{
		{name: "流式窗口到点", ctx: context.Background(), err: headerTimeout,
			window:    upstreamWindow{Kind: upstreamWindowStream, Window: 30 * time.Second},
			wantParts: []string{"流式等头窗口", "30 秒", headerWindowTimeoutMarker}},
		{name: "非流式窗口到点", ctx: context.Background(), err: headerTimeout,
			window:    upstreamWindow{Kind: upstreamWindowNonStream, Window: 90 * time.Second},
			wantParts: []string{"非流式整段预算窗口", "90 秒", headerWindowTimeoutMarker}},
		{name: "上游自己断连", ctx: context.Background(), err: upstreamReset,
			window: upstreamWindow{Kind: upstreamWindowNonStream, Window: 90 * time.Second}},
		{name: "连接调优超时非我们的窗口", ctx: context.Background(), err: dialTimeout,
			window: upstreamWindow{Kind: upstreamWindowNonStream, Window: 90 * time.Second}},
		{name: "请求ctx已结束（总超时/撤走族）", ctx: expiredCtx, err: headerTimeout,
			window: upstreamWindow{Kind: upstreamWindowNonStream, Window: 90 * time.Second}},
		{name: "窗口未知", ctx: context.Background(), err: headerTimeout, window: upstreamWindow{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := describeUpstreamWindowTimeout(tc.ctx, tc.err, tc.window)
			if tc.wantParts == nil {
				if got.Error() != tc.err.Error() {
					t.Fatalf("got %q, want 原样返回 %q（不贴类别）", got.Error(), tc.err.Error())
				}
				return
			}
			for _, part := range tc.wantParts {
				if !strings.Contains(got.Error(), part) {
					t.Fatalf("got %q, want 含 %q", got.Error(), part)
				}
			}
			// 类别前缀不得切断错误链：断连/超时归因都按 errors.Is/As 匹配。
			var netErr net.Error
			if !errors.As(got, &netErr) {
				t.Fatalf("got %q, want 仍可 errors.As 到 net.Error（错误链未被切断）", got.Error())
			}
		})
	}
}
