package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// splitWindowSettingsReader 让两个窗口取到**可区分**的值（流式等头 vs 非流式整段）。
// 取可区分值是关键：两者同值时，分不出「按 stream 分流」与「沿用旧单一窗口」。
type splitWindowSettingsReader struct {
	streamWindowSec    int
	nonStreamWindowSec int
	maxRetry           int
}

func (r splitWindowSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (r splitWindowSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	switch key {
	case models.SettingKeyRequestHeaderTimeout:
		return r.streamWindowSec
	case models.SettingKeyNonStreamResponseTimeout:
		return r.nonStreamWindowSec
	case models.SettingKeyRequestMaxRetry:
		return r.maxRetry
	}
	return def
}

func (r splitWindowSettingsReader) String(_ context.Context, _ string, def string) string { return def }

func (r splitWindowSettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	return r.Bool(ctx, key, def), nil
}

func (r splitWindowSettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	return r.Int(ctx, key, def, min), nil
}

// TestUpstreamAttemptWindow_SplitsByStream 锁分流判据本身：同一份设置下，两种入站形态
// 取到不同窗口。变异：resolver 忽略 stream 参数、恒取响应头键 → 红。
func TestUpstreamAttemptWindow_SplitsByStream(t *testing.T) {
	SetSettingsReader(splitWindowSettingsReader{streamWindowSec: 30, nonStreamWindowSec: 90, maxRetry: 1})
	t.Cleanup(func() { SetSettingsReader(nil) })

	if got := upstreamAttemptWindow(context.Background(), true); got != 30*time.Second {
		t.Fatalf("流式窗口 = %v, want 30s（等上游响应头）", got)
	}
	if got := upstreamAttemptWindow(context.Background(), false); got != 90*time.Second {
		t.Fatalf("非流式窗口 = %v, want 90s（等整段回答，等价整段生成预算）", got)
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
	SetSettingsReader(splitWindowSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 3, maxRetry: 1})
	t.Cleanup(func() { SetSettingsReader(nil) })

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
	SetSettingsReader(splitWindowSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 3, maxRetry: 1})
	t.Cleanup(func() { SetSettingsReader(nil) })

	_, err := balanceOnce(t, pool, model.Name, true)
	if err == nil {
		t.Fatal("err = nil, want 等头窗口超时失败（流式窗口未被放宽）")
	}
	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusBadGateway {
		t.Fatalf("err = %v, want 502（换家换不到仍是「上游全挂」口径）", err)
	}
}
