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
// 端点/分组/凭据/关联），返回候选池。慢上游在 ctx deadline 到点时让 Client.Do 以
// deadline 错误落地，模拟「网关总超时掐断」。
func setupTimeoutProbeProvider(t *testing.T, sleep time.Duration) (CandidatePool, string) {
	t.Helper()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
	}))
	t.Cleanup(slow.Close)

	model := models.Model{Name: "m-timeout"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "slow-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, slow.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	addLegacyChannelRows(t, cipher, provider)
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "slow-upstream", Weight: 10, Priority: 10}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	return pool, model.Name
}

// TestBalanceChat_DeadlineCtx_GatewayTimeout504 总超时到点 → 504（不是裸 deadline 错误、
// 不是 500）：网关自己掐断要有可辨识的对外状态码。
func TestBalanceChat_DeadlineCtx_GatewayTimeout504(t *testing.T) {
	pool, modelName := setupTimeoutProbeProvider(t, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: modelName, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, modelName))},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: modelName},
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
	pool, modelName := setupTimeoutProbeProvider(t, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := BalanceChat(ctx, BalanceInput{
		Start: time.Now(),
		Style: "openai",
		Before: Before{Model: modelName, Stream: false,
			raw: []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, modelName))},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: modelName},
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
func TestWithRequestTotalTimeout_DerivesDeadline(t *testing.T) {
	SetSettingsReader(timeoutSettingsReader{})
	t.Cleanup(func() { SetSettingsReader(nil) })

	ctx, cancel := WithRequestTotalTimeout(context.Background())
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("派生 ctx 无 deadline，want 带网关总超时 deadline")
	}
	// timeoutSettingsReader 的总超时键为 3 秒
	if got := time.Until(dl); got < 2*time.Second || got > 4*time.Second {
		t.Fatalf("deadline 剩余 %v, want ≈3s（取自设置项）", got)
	}
}
