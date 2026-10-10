package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// 本文件是 Stage 33「网关总超时落请求 ctx」的回归组：总超时到点 = 服务端掐断，
// 归因是「超时」不是「调用方撤走」，对外回 504，结算照常（按供应商失败处置）。

// TestIsClientDisconnectErr_TimeoutNotClientDisconnect 归因锁：超时（deadline /
// 网络超时）不是「调用方撤走」，不得被断连降级口径吞掉（否则超时被记成放弃）。
func TestIsClientDisconnectErr_TimeoutNotClientDisconnect(t *testing.T) {
	if isClientDisconnectErr(context.DeadlineExceeded) {
		t.Fatal("DeadlineExceeded 被判成调用方断连，want 非断连（超时归因被吞）")
	}
	var netTimeout netTimeoutError
	if isClientDisconnectErr(netTimeout) {
		t.Fatal("网络超时错误被判成调用方断连，want 非断连")
	}
	if !isClientDisconnectErr(context.Canceled) {
		t.Fatal("context.Canceled 未被判成断连，want 断连（既有语义回归）")
	}
}

// netTimeoutError 实现 net.Error 的 Timeout 形态（模拟 http 客户端超时错误）。
type netTimeoutError struct{}

func (netTimeoutError) Error() string   { return "simulated net timeout" }
func (netTimeoutError) Timeout() bool   { return true }
func (netTimeoutError) Temporary() bool { return false }

// setupTimeoutProbeProvider 建一个「不回响应头」的慢上游 + 最小可用路由（模型/供应商/
// 端点/分组/凭据/关联），返回候选池与真实模型行（虚拟路径用例需要模型 ID 走
// queryEnabledModelProviders）。慢上游在 ctx deadline 到点时让 Client.Do 以
// deadline 错误落地，模拟「网关总超时掐断」。关联置启用：虚拟路径按
// ListEnabledByModelID（status=true）取候选，停用关联会被整体跳过。
func setupTimeoutProbeProvider(t *testing.T, sleep time.Duration) (CandidatePool, models.Model) {
	t.Helper()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
	}))
	t.Cleanup(slow.Close)

	enabled := true
	model := models.Model{Name: "m-timeout"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "slow-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, slow.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	addLegacyChannelRows(t, cipher, provider)
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

// TestBalanceChat_DeadlineCtx_GatewayTimeout504 总超时到点 → 504（不是裸 deadline 错误、
// 不是 500）：网关自己掐断要有可辨识的对外状态码。
func TestBalanceChat_DeadlineCtx_GatewayTimeout504(t *testing.T) {
	pool, model := setupTimeoutProbeProvider(t, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: model.Name, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model.Name))},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: model.Name},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("err = nil, want 超时错误")
	}
	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusGatewayTimeout {
		t.Fatalf("err = %v, want 自带 504 的超时错误（网关掐断的可辨识归因）", err)
	}
}

// TestBalanceChat_DeadlineCtx_SettlesAsProviderFailure 超时不是「调用方撤走」：
// 结算照常（凭据冷却写入），慢供应商的调度信号不丢（Stage 33 AC-4 的结算半边）。
func TestBalanceChat_DeadlineCtx_SettlesAsProviderFailure(t *testing.T) {
	pool, model := setupTimeoutProbeProvider(t, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: model.Name, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model.Name))},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: model.Name},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("err = nil, want 超时错误")
	}

	var creds []models.Credential
	if err := models.DB.Find(&creds).Error; err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if len(creds) == 0 {
		t.Fatal("no credential rows（setup 未建凭据）")
	}
	// 冷却写经 credwrite 有界队列异步执行：测试同步模式下这里应已落库。
	// 若未落库即「超时按放弃处置、不冷却」——恰是要防的调度信号丢失。
	settled := false
	for _, c := range creds {
		if c.CooldownUntil != nil {
			settled = true
		}
	}
	if !settled {
		t.Fatal("无任何凭据被写冷却，want 超时按供应商失败结算（写冷却）")
	}
}

// TestWithRequestTotalTimeout_DerivesDeadline 锁「总超时值 → 请求 ctx deadline」的派生：
// handler 入口经此挂网关总超时，到点全请求（含流式输出）受控。
// 桩必须**显式**给总超时键：Stage 38 起该桩是「字段 0 就回退生产默认值」的参数化桩
// （此前是固定值桩，total 键恒 3），漏填会静默取 600 让本用例失去判据。
func TestWithRequestTotalTimeout_DerivesDeadline(t *testing.T) {
	installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 1, totalTimeoutSec: 3, maxRetry: 2})

	ctx, cancel := WithRequestTotalTimeout(context.Background())
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("派生 ctx 无 deadline，want 带网关总超时 deadline")
	}
	// 上面显式给的总超时键 = 3 秒
	if got := time.Until(dl); got < 2*time.Second || got > 4*time.Second {
		t.Fatalf("deadline 剩余 %v, want ≈3s（取自设置项）", got)
	}
}

// TestBalanceChatVirtual_DeadlineCtx_GatewayTimeout504 虚拟路径的网关掐断归因锁（审查
// P0-1 回归）：总超时到点时虚拟路径同样回自带 504 的超时错误——归因映射收口到
// BalanceChat 唯一出口前，虚拟路径的中止出口曾裸抛 DeadlineExceeded，对外成 500。
func TestBalanceChatVirtual_DeadlineCtx_GatewayTimeout504(t *testing.T) {
	_, realModel := setupTimeoutProbeProvider(t, 3*time.Second)

	virtual := models.VirtualModel{Name: "vm-timeout", Strategy: "priority"}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: virtual.Name, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, virtual.Name))},
		ProvidersWithMeta: ProvidersWithMeta{
			IsVirtualModel:    true,
			VirtualModelID:    virtual.ID,
			VirtualModelName:  virtual.Name,
			VirtualStrategy:   virtual.Strategy,
			OrderedRealModels: []virtualmodel.OrderedRealModel{{Model: realModel}},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("err = nil, want 超时错误")
	}
	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusGatewayTimeout {
		t.Fatalf("err = %v, want 自带 504 的超时错误（虚拟路径出口归因收口）", err)
	}
}

// singleRetrySettingsReader 测试 stub：MaxRetry=1（末次尝试即穷尽），其余键回退默认值。
type singleRetrySettingsReader struct{}

func (singleRetrySettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (singleRetrySettingsReader) Int(_ context.Context, key string, def, _ int) int {
	if key == models.SettingKeyRequestMaxRetry {
		return 1
	}
	return def
}

func (singleRetrySettingsReader) String(_ context.Context, _ string, def string) string { return def }

func (r singleRetrySettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	return r.Bool(ctx, key, def), nil
}

func (r singleRetrySettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	return r.Int(ctx, key, def, min), nil
}

// TestBalanceChat_LastAttemptHitsDeadline_ExhaustedMapsTo504 穷尽出口撞超时的归因修正
// （审查 P2-2 回归）：MaxRetry=1 且末次尝试恰被总超时掐断时，重试循环以 exhausted 收场
// （已无剩余迭代可走中止分支），本质是网关掐断——必须回 504；记「上游全挂」的 502
// 会让监控把网关超时当成上游故障。
func TestBalanceChat_LastAttemptHitsDeadline_ExhaustedMapsTo504(t *testing.T) {
	pool, model := setupTimeoutProbeProvider(t, 3*time.Second)
	installChatTestSettings(t, singleRetrySettingsReader{})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: model.Name, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model.Name))},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: model.Name},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("err = nil, want 超时错误")
	}
	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusGatewayTimeout {
		t.Fatalf("err = %v, want 自带 504 的超时错误（穷尽撞 deadline 归超时；回 502 即穷尽出口缺归因修正）", err)
	}
}

// TestMapAbortErr_CanceledPassesThrough 归因锁的另一半：调用方撤走（Canceled）必须原样
// 透传——ctx 已死说明对面多半已收不到，包成 504 反而把用户行为记成网关故障。
func TestMapAbortErr_CanceledPassesThrough(t *testing.T) {
	if got := mapAbortErr(context.Canceled); !errors.Is(got, context.Canceled) {
		t.Fatalf("mapAbortErr(Canceled) = %v, want 原样透传", got)
	}
	if got := mapAbortErr(nil); got != nil {
		t.Fatalf("mapAbortErr(nil) = %v, want nil", got)
	}

	mapped := mapAbortErr(fmt.Errorf("attempt aborted: %w", context.DeadlineExceeded))
	var statusCoder interface{ StatusCode() int }
	if !errors.As(mapped, &statusCoder) || statusCoder.StatusCode() != http.StatusGatewayTimeout {
		t.Fatalf("mapAbortErr(wrapped deadline) = %v, want 自带 504 的超时错误（包装错误也归超时）", mapped)
	}
}

// TestRetryExhaustedOutcome_DeadlineCtxYieldsTimeout 穷尽出口归因修正的单测半边：
// ctx 已到 deadline 时不得回 502「上游全挂」，须交由 mapAbortErr 按超时归因成 504；
// ctx 仍活时穷尽照旧是 502。
func TestRetryExhaustedOutcome_DeadlineCtxYieldsTimeout(t *testing.T) {
	dead, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-dead.Done()

	err := retryExhaustedOutcome(dead, "maximum retry attempts reached")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded（经唯一出口映射成 504）", err)
	}
	var statusCoder interface{ StatusCode() int }
	if errors.As(err, &statusCoder) {
		t.Fatalf("err = %v, want 不带穷尽的 502 状态码", err)
	}

	err = retryExhaustedOutcome(context.Background(), "maximum retry attempts reached")
	if !errors.As(err, &statusCoder) || statusCoder.StatusCode() != http.StatusBadGateway {
		t.Fatalf("err = %v, want 自带 502 的穷尽错误", err)
	}
}
