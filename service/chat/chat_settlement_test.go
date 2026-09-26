package chat

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// reloadModelWithProvider 读回关联行（结算断言用）。
func reloadModelWithProvider(t *testing.T, id uint) models.ModelWithProvider {
	t.Helper()
	var got models.ModelWithProvider
	if err := models.DB.First(&got, id).Error; err != nil {
		t.Fatalf("reload model-provider: %v", err)
	}
	return got
}

// seedWeight 显式设权重：关联默认权重为 0，而衰减以 floor=1 为下界
// （weight > floor 才写），不设初值的话「降权」分支恒不命中。
func seedWeight(t *testing.T, id uint, weight int) {
	t.Helper()
	if err := models.DB.Model(&models.ModelWithProvider{}).Where("id = ?", id).Update("weight", weight).Error; err != nil {
		t.Fatalf("seed weight: %v", err)
	}
}

// enableAutoWeightDecay 打开自动权重衰减：adjustment 包读的是它自己的 settings.Reader
// （默认委托 models.GetSetting*），故此处改 DB 里的设置行而非注入 chat 包 stub。
// 建库时 schema 已种默认值，只改值——再 Insert 会撞唯一索引。
func enableAutoWeightDecay(t *testing.T) {
	t.Helper()
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyAutoWeightDecay).
		Update("value", "true").Error; err != nil {
		t.Fatalf("enable auto weight decay: %v", err)
	}
}

// Stage 24 AC-3 区分性判据：只有「调用方撤走」（请求 ctx 取消）才算放弃；本服务等到
// 超时（deadline）是上游不吭声的真实故障，照常结算。防「见 ctx 死就一刀切不结算」。
func TestRequestAbandoned_OnlyCanceledSignal(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelExpired()
	<-expired.Done()

	cases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{"活着的请求 ctx", context.Background(), false},
		{"调用方撤走（取消）", canceled, true},
		{"本服务等到超时（deadline）", expired, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := requestAbandoned(tc.ctx); got != tc.want {
				t.Fatalf("requestAbandoned = %v, want %v", got, tc.want)
			}
		})
	}
}

// Stage 24 AC-2「该记的没记上」半边：请求 ctx 因本服务超时（deadline）而死时，结算仍
// 必须落库——坏供应商不被降权、坏 key 不被避让，恰在最需要保护的故障期失效。
// 样本取超限出口（组织级结算：连续失败计数 + 权重衰减）。改前实测红：写库直接用了已死的
// 请求 ctx，GORM 报错后只留一行 slog，DB 一个字段都没变。
func TestHandleUpstreamBodyOverflow_DeadlineExpiredCtxStillSettles(t *testing.T) {
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	enableAutoWeightDecay(t)

	input := makeInput("settle-deadline")
	seedWeight(t, input.ModelWithProvider.ID, 10)

	deadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	<-deadCtx.Done() // 模拟「本服务等到超时」：请求 ctx 已死，但这次失败是真实的上游/端点故障
	input.Ctx = deadCtx

	before := reloadModelWithProvider(t, input.ModelWithProvider.ID)
	result := handleUpstreamBodyOverflow(input, &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, 0,
		models.RawLogOptions{}, requestLogSnapshot{}, errUpstreamBodyOverflow)
	if result.FatalErr != nil {
		t.Fatalf("超时不是调用方放弃，不得走中止路径: %v", result.FatalErr)
	}

	after := reloadModelWithProvider(t, input.ModelWithProvider.ID)
	if after.ConsecutiveFailures != before.ConsecutiveFailures+1 {
		t.Fatalf("ConsecutiveFailures = %d, want %d（结算写库不得陪葬请求 ctx）",
			after.ConsecutiveFailures, before.ConsecutiveFailures+1)
	}
	if after.Weight != before.Weight-1 {
		t.Fatalf("Weight = %d, want %d（自动衰减开启时超时失败必须降权）", after.Weight, before.Weight-1)
	}
}

// Stage 24 AC-1 组织级出口：调用方撤走时不结算（不累计连续失败、不降权），且请求立即
// 中止而不是继续换候选。
func TestHandleUpstreamBodyOverflow_CallerAbandonedNoSettlement(t *testing.T) {
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	enableAutoWeightDecay(t)

	input := makeInput("abandon-overflow")
	seedWeight(t, input.ModelWithProvider.ID, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 调用方已撤走
	input.Ctx = ctx

	before := reloadModelWithProvider(t, input.ModelWithProvider.ID)
	result := handleUpstreamBodyOverflow(input, &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, 0,
		models.RawLogOptions{}, requestLogSnapshot{}, errUpstreamBodyOverflow)
	if result.FatalErr == nil {
		t.Fatal("调用方放弃后应中止本次请求（FatalErr 非空），不再换候选")
	}
	after := reloadModelWithProvider(t, input.ModelWithProvider.ID)
	if after.ConsecutiveFailures != before.ConsecutiveFailures || after.Weight != before.Weight {
		t.Fatalf("调用方放弃不得结算：ConsecutiveFailures %d→%d, Weight %d→%d",
			before.ConsecutiveFailures, after.ConsecutiveFailures, before.Weight, after.Weight)
	}
}

// assertNoCredentialSettlement 断言选中凭据未被冷却、未被记失败、未被推进判停。
func assertNoCredentialSettlement(t *testing.T, credentialID uint) {
	t.Helper()
	got := reloadCredentialStats(t, credentialID)
	if got.CooldownUntil != nil {
		t.Fatalf("CooldownUntil = %v, want nil（无辜 key 不得被冷却）", got.CooldownUntil)
	}
	if got.CooldownReason != "" {
		t.Fatalf("CooldownReason = %q, want 空（不得冷却）", got.CooldownReason)
	}
	if got.TotalErrors != 0 {
		t.Fatalf("TotalErrors = %d, want 0（无辜 key 不得记失败）", got.TotalErrors)
	}
	if got.FailCount != 0 {
		t.Fatalf("FailCount = %d, want 0（不得推进鉴权判停）", got.FailCount)
	}
}

// runAttemptUntilCallerAbandons 跑一次 attempt，等上游 handler 开始处理后让调用方撤走
// （ctx 取消），返回 attempt 结果。
// 上游 handler 用 started 通知「已收到请求」、用 release 放行收尾：服务端**不能**靠
// 「等 r.Context().Done()」感知断连——无读写进行时 http.Server 不会察觉对端关闭
// （实测会挂到测试超时）。
func runAttemptUntilCallerAbandons(t *testing.T, input singleProviderAttemptInput, cancel context.CancelFunc, started, release chan struct{}) singleProviderAttemptResult {
	t.Helper()
	resultCh := make(chan singleProviderAttemptResult, 1)
	go func() { resultCh <- executeSingleProviderAttempt(input, make(chan models.ChatLog)) }()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("上游 handler 未在预期时间内开始处理请求")
	}
	cancel() // 调用方撤走（客户端断连）

	var result singleProviderAttemptResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("调用方撤走后 attempt 未及时结束")
	}
	close(release) // 放行上游 handler，避免 srv.Close 等它
	return result
}

// Stage 24 AC-1 出口之一（上游请求阶段）：请求已发出后客户端撤走 → 不冷却、不计失败，
// 只留失败日志（终态 error），并立即中止。
func TestExecuteSingleProviderAttempt_CallerDisconnect_NoCredentialSettlement(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // 拖住上游：本次请求永远得不到响应
	}))

	input := makeInput("disconnect-during-do")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input.Ctx = ctx

	result := runAttemptUntilCallerAbandons(t, input, cancel, started, release)
	if result.FatalErr == nil {
		t.Fatalf("调用方撤走后应中止本次请求，result = %+v", result)
	}
	if result.CredentialFailure {
		t.Fatalf("调用方撤走不是凭据级失败，不得标 CredentialFailure: %+v", result)
	}
	assertNoCredentialSettlement(t, input.Selection.Credential.ID)

	// 留痕仍然写：失败终态不得陪葬请求 ctx（Stage 17 口径）。
	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&row).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if row.Status != "error" {
		t.Fatalf("日志行终态 = %q, want error（失败留痕照写）", row.Status)
	}
}

// Stage 24 AC-1 出口之二（读上游回包阶段）：headers 已到、body 滴灌中客户端撤走 →
// 读体失败归凭据级网络出口，但这次失败不是 key 的错——不冷却、不计失败。
func TestExecuteSingleProviderAttempt_CallerDisconnectDuringRead_NoCredentialSettlement(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // 头先到达：把失败点逼进读体阶段
		close(started)
		<-release
	}))

	input := makeInput("disconnect-during-read")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input.Ctx = ctx

	result := runAttemptUntilCallerAbandons(t, input, cancel, started, release)
	if result.FatalErr == nil {
		t.Fatalf("调用方撤走后应中止本次请求，result = %+v", result)
	}
	assertNoCredentialSettlement(t, input.Selection.Credential.ID)
}

// Stage 24 AC-1 出口之三（提交门预读）：流式请求头已到、首帧始终不来时客户端撤走
// ——这正是本 Stage 的来源场景（断连打断预读不得冷却无辜 key）。
func TestExecuteSingleProviderAttempt_CallerDisconnectDuringStreamGate_NoCredentialSettlement(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // SSE 头已提交、一帧不发
		close(started)
		<-release
	}))

	input := makeInput("disconnect-during-gate")
	input.Before.Stream = true
	input.Before.raw = []byte(`{"model":"m1","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input.Ctx = ctx

	result := runAttemptUntilCallerAbandons(t, input, cancel, started, release)
	if result.FatalErr == nil {
		t.Fatalf("调用方撤走后应中止本次请求（提交门预读被打断），result = %+v", result)
	}
	assertNoCredentialSettlement(t, input.Selection.Credential.ID)
}

// Stage 24 AC-4 成功侧机制锁：上游已经成功是既成事实，成功结算（计数清零/记账）不因
// 调用方恰在此刻撤走而作废。锁「结算 ctx 与请求生死解耦」这条缝本身（不是红测——
// 该窗口在集成层造不出来：成功路径只在上游真返回成功时到达）。
func TestSuccessWriteCtx_SettlesAfterRequestCtxDead(t *testing.T) {
	initChatRecordTestDB(t)

	cred := models.Credential{Key: "enc", KeyHash: "hash", FailCount: 2}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	deadCtx, cancel := context.WithCancel(context.Background())
	cancel() // 请求 ctx 已死（调用方恰在收尾瞬间撤走）
	successCtx, cancelSuccess := successWriteCtx(deadCtx)
	defer cancelSuccess()

	resetCredentialAuthFailCount(successCtx, cred)

	if got := reloadCredentialStats(t, cred.ID); got.FailCount != 0 {
		t.Fatalf("FailCount = %d, want 0（成功结算不得陪葬请求 ctx）", got.FailCount)
	}
}
