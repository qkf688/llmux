package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

// testChatCredHexKey 与 service/channel 测试同值的 32 字节 hex 加密密钥
// （凭据选择需要 decrypt 明文 key）。
const testChatCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestRunProviderRetryLoop(t *testing.T) {
	initChatRecordTestDB(t)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// baseInput 构造「两个候选、DB 均无端点/分组/凭据」的候选池：选路（channel Select）
	// 必然失败。各用例只替换要验证的那一项，其余保持基准，避免每个场景复制一整份入参。
	baseInput := func() retryLoopInput {
		return retryLoopInput{
			Ctx:    context.Background(),
			Start:  time.Now(),
			Style:  "openai",
			Before: Before{Model: "m"},
			Pool: CandidatePool{
				ProviderMap: map[uint]models.Provider{
					11: {Type: "no-such-provider-type", Name: "bad-1"},
					12: {Type: "no-such-provider-type", Name: "bad-2"},
				},
				// 候选身份由 map key 标识（循环经 key 查找并淘汰），无需填充嵌入的 gorm.Model.ID。
				ModelWithProviderMap: map[uint]models.ModelWithProvider{
					1: {ProviderID: 11, ProviderModel: "m-1"},
					2: {ProviderID: 12, ProviderModel: "m-2"},
				},
				WeightItems:   map[uint]int{1: 10, 2: 10},
				PriorityItems: map[uint]int{1: 1, 2: 1},
			},
			RealModelName: "m",
			MaxRetry:      5,
			ClientTimeout: time.Second,
			// 不触发的超时通道：本组用例只关心候选淘汰与取消路径。
			Deadline:    make(chan time.Time),
			DeadlineErr: errors.New("unused"),
		}
	}

	tests := []struct {
		name       string
		mutate     func(*retryLoopInput)
		wantStatus retryLoopStatus
		wantErr    error
	}{
		{
			// 候选无选路数据（端点/分组/凭据全缺）时只淘汰该候选，池中其余候选
			// 继续尝试，直到候选耗尽才报穷尽——不得升级为致命错误终止整个请求。
			name:       "unusable providers skipped until pool exhausted",
			wantStatus: retryLoopExhausted,
		},
		{
			// ctx 取消须原样上抛 ctx.Err()，不得降级为「候选池穷尽」（后者会让虚拟路径继续换模型）。
			name:       "canceled context aborts",
			mutate:     func(in *retryLoopInput) { in.Ctx = canceledCtx },
			wantStatus: retryLoopAborted,
			wantErr:    context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			outcome := runProviderRetryLoop(in)

			if outcome.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d; err = %v", outcome.Status, tt.wantStatus, outcome.Err)
			}
			if tt.wantErr != nil && !errors.Is(outcome.Err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", outcome.Err, tt.wantErr)
			}
		})
	}
}

// TestRunProviderRetryLoop_SkipsUnselectableProviderThenSucceeds：
// 池中候选「无端点/分组/凭据数据（选路必失败）」时只淘汰该候选继续尝试，
// 不得升级为 FatalErr；剩余正常候选仍应完成请求——选路失败即组织级淘汰
// （#13 才做凭据级组内转移，此处不区分 sentinel 层级）。
func TestRunProviderRetryLoop_SkipsUnselectableProviderThenSucceeds(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var upstreamHit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit.Add(1)
		// bad 候选若发出请求（旧行为：绕过选路直连），必带 model=pm-bad → 502 淘汰；
		// good 命中的请求带 pm-good → 200。计数 1 次 = 只有 good 被放行。
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "pm-bad") {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"pm-good","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	// good 供应商：1 端点（openai、URL 空继承 base_url）+ 1 分组 + 1 内联凭据——
	// 老 Provider 迁移后的最小等价形态，选路必须成功。
	good := models.Provider{
		Name:   "good",
		Type:   "openai",
		Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL),
	}
	if err := models.DB.Create(&good).Error; err != nil {
		t.Fatalf("create good provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: good.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: good.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plainKey := "sk-good-key"
	encKey, err := cipher.Encrypt(plainKey)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: encKey, KeyHash: cipher.Hash(plainKey)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	goodMWP := models.ModelWithProvider{ProviderID: 12, ProviderModel: "pm-good"}
	if err := models.DB.Create(&goodMWP).Error; err != nil {
		t.Fatalf("create good assoc: %v", err)
	}
	// bad 的 MWP 也落库：Adjustment 按 MWP.ID 自增/复位连续失败（re-read 需要行存在），
	// 不落库只会留下 record not found 噪音——ProviderID 11 无 DB 行（候选身份由 map 标识）。
	badMWP := models.ModelWithProvider{ProviderID: 11, ProviderModel: "pm-bad"}
	if err := models.DB.Create(&badMWP).Error; err != nil {
		t.Fatalf("create bad assoc: %v", err)
	}

	// bad 候选：仅存在于候选池 map，DB 无其端点/分组/凭据 → 选路必失败被淘汰。
	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now(),
		Style:         string(consts.StyleOpenAI),
		Before:        Before{Model: "m", Stream: false, raw: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName: "m",
		Pool: CandidatePool{
			ProviderMap: map[uint]models.Provider{
				// bad 候选给 base_url 指向 srv：若绕过选路（旧行为）必打中 502 分支被淘汰。
				11: {Name: "bad-no-data", Type: "openai", Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)},
				12: good,
			},
			ModelWithProviderMap: map[uint]models.ModelWithProvider{
				badMWP.ID:  badMWP,
				goodMWP.ID: goodMWP,
			},
			WeightItems:   map[uint]int{badMWP.ID: 10, goodMWP.ID: 10},
			PriorityItems: map[uint]int{badMWP.ID: 1, goodMWP.ID: 1},
		},
		MaxRetry:      5,
		ClientTimeout: 2 * time.Second,
		Deadline:      make(chan time.Time),
		DeadlineErr:   errors.New("unused"),
	})

	if outcome.Status != retryLoopSucceeded {
		t.Fatalf("status = %d, want %d; err = %v", outcome.Status, retryLoopSucceeded, outcome.Err)
	}
	if got := upstreamHit.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1（bad 候选必须被选路提前拦截，只有 good 发出请求）", got)
	}
}

// TestRunProviderRetryLoop_CredentialsExhausted_ProbeRecoverInGroup 锁 #12 内层
// RetryCredential 耗尽探活端到端：组内首条 active key 401 判停后，锁组 RetryCredential
// 落入全 temp_unsched 组 → 惰性探活（仅 key2 过频控且探活成功）→ 刷新快照锁组重选
// key2 → 换 key 重试成功。设计上与既有差异点：
//
//   - 与 TestBalanceChat_CredentialsExhausted_OrganizationFailure（探活缺席 → 组织级失败）
//     对照，本用例是探活**恢复成功**的正向半边；
//   - key1 LastProbeAt 设在频控窗口内：401 判停后探活候选按 ID ASC 会先看 key1，
//     若放过它 mock 探活返回 500 会让整个恢复路径消失——锁「探活候选必须跳过频控内 key」；
//   - 探活只打 key2 一次：本链只发生单次耗尽事件，重复探活由 credprobe 频控/CAS
//     （pickProbeCandidate + ClaimLastProbeAt）天然挡住；循环内 probedThisCandidate
//     旗标锁「同一候选二次耗尽禁探」，集成层难以构造该分支（恢复后的凭据不再是
//     temp_unsched，pickProbeCandidate 不会再次命中），不在本测试断言范围。
func TestRunProviderRetryLoop_CredentialsExhausted_ProbeRecoverInGroup(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	SetSettingsReader(credCooldownSettingsReader{
		Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 1, ProbeIntervalSec: 60,
	})
	t.Cleanup(func() { SetSettingsReader(nil) })

	var mu sync.Mutex
	var chatAuths, probeAuths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// 探活路径：GET {base}/models。key2 成功、其它失败——若探活落到 key1
			// （频控失效）本用例会直接走组织级失败分支，观测不到恢复路径。
			mu.Lock()
			probeAuths = append(probeAuths, auth)
			mu.Unlock()
			if auth == "Bearer sk-key2" {
				_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"probe boom"}`))
			return
		}
		// 聊天路径：POST {base}/chat/completions。key1 401 判停，key2 200 成功。
		mu.Lock()
		chatAuths = append(chatAuths, auth)
		mu.Unlock()
		if auth == "Bearer sk-key1" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid key"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatSuccessBody))
	}))
	defer srv.Close()

	provider := models.Provider{Name: "probe-loop", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	// key1：active 首条命中；LastProbeAt 设在 60s 频控窗口内（now-30s），
	// 401 判停后探活候选跳过它，保证探活目标收敛到 key2。
	probedRecently := time.Now().Add(-30 * time.Second)
	key1 := models.Credential{
		GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key1"), KeyHash: cipher.Hash("sk-key1"),
		Status: models.CredentialStatusActive, LastProbeAt: &probedRecently,
	}
	// key2：已判停等待探活恢复（探活候选 ID ASC，key1 频控跳过后才轮到它）。
	key2 := models.Credential{
		GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key2"), KeyHash: cipher.Hash("sk-key2"),
		Status: models.CredentialStatusTempUnsched, CooldownReason: "auth_fail", FailCount: 3,
	}
	for _, cred := range []*models.Credential{&key1, &key2} {
		if err := models.DB.Create(cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
	}
	mwp := models.ModelWithProvider{ProviderID: provider.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now(),
		Style:         string(consts.StyleOpenAI),
		Before:        Before{Model: "m", Stream: false, raw: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName: "m",
		Pool: CandidatePool{
			ProviderMap: map[uint]models.Provider{
				provider.ID: provider,
			},
			ModelWithProviderMap: map[uint]models.ModelWithProvider{
				mwp.ID: mwp,
			},
			WeightItems:   map[uint]int{mwp.ID: 10},
			PriorityItems: map[uint]int{mwp.ID: 1},
		},
		MaxRetry:      5,
		ClientTimeout: 2 * time.Second,
		Deadline:      make(chan time.Time),
		DeadlineErr:   errors.New("unused"),
	})

	if outcome.Status != retryLoopSucceeded {
		t.Fatalf("status = %d, want %d; err = %v", outcome.Status, retryLoopSucceeded, outcome.Err)
	}

	mu.Lock()
	if len(chatAuths) != 2 || chatAuths[0] != "Bearer sk-key1" || chatAuths[1] != "Bearer sk-key2" {
		mu.Unlock()
		t.Fatalf("chat auth sequence = %v, want [Bearer sk-key1 Bearer sk-key2]（key1 401 判停后探活恢复 key2 换 key 成功）", chatAuths)
	}
	if len(probeAuths) != 1 || probeAuths[0] != "Bearer sk-key2" {
		mu.Unlock()
		t.Fatalf("probe auths = %v, want [Bearer sk-key2]（只发生一次耗尽，探活仅打 key2 一次）", probeAuths)
	}
	mu.Unlock()

	// key1：401 达阈值（AuthFailThreshold=1）判停 temp_unsched
	var key1Reload models.Credential
	if err := models.DB.First(&key1Reload, key1.ID).Error; err != nil {
		t.Fatalf("reload key1: %v", err)
	}
	if key1Reload.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("key1 Status = %q, want temp_unsched（401 判停）", key1Reload.Status)
	}
	// key2：探活成功条件恢复 active + 计数清零 + 清 reason（#6-3 恢复写路径）
	var key2Reload models.Credential
	if err := models.DB.First(&key2Reload, key2.ID).Error; err != nil {
		t.Fatalf("reload key2: %v", err)
	}
	if key2Reload.Status != models.CredentialStatusActive {
		t.Fatalf("key2 Status = %q, want active（探活成功恢复）", key2Reload.Status)
	}
	if key2Reload.FailCount != 0 || key2Reload.CooldownReason != "" {
		t.Fatalf("key2 recovered fields = fail=%d reason=%q, want 0/空", key2Reload.FailCount, key2Reload.CooldownReason)
	}

	// 组内探活恢复成功 = 凭据级消化，**不**断言组织级 ConsecutiveFailures：本用例以成功
	// 告终，attempt 成功出口无条件重置该字段，断言恒为 0 无法捕获任何回归（假绿）；
	// 「层内耗尽才累计组织级」的对照语义由负向用例
	// TestRunProviderRetryLoop_CredentialsExhausted_ProbeFailsOrganizationFailure 在失败
	// 结局中断言 1——那里没有成功出口重置，真实可观测。
}

// TestRunProviderRetryLoop_CredentialsExhausted_ProbeFailsOrganizationFailure 锁 #12
// 内层耗尽的**负半边**（与上面的正向用例对照）：组内首条 active key 401 判停后，锁组
// RetryCredential 落入全 temp_unsched 组 → 惰性探活 key2 但上游 /models 返回 500 →
// 探活失败仅 touch（status 不恢复、时间被 stamp）→ 不进入恢复分支 → 统一在循环外做
// 组织级处置（候选移除 + ConsecutiveFailures 补累计一次）→ 后续外层选路池空 → 穷尽。
// 锁的语义：恢复成功=凭据级消化不累计（正向），恢复失败=层内耗尽补累计（本用例）——
// 两级边界在失败结局才可观测（成功出口会无条件重置，见正向用例尾注释）。
func TestRunProviderRetryLoop_CredentialsExhausted_ProbeFailsOrganizationFailure(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	SetSettingsReader(credCooldownSettingsReader{
		Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 1, ProbeIntervalSec: 60,
	})
	t.Cleanup(func() { SetSettingsReader(nil) })

	var mu sync.Mutex
	var chatAuths, probeAuths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// 探活路径：任何 key 都 500（探活失败）——key1 本就在频控窗口内被跳过，
			// key2 是唯一探活候选，失败后保持 temp_unsched。
			mu.Lock()
			probeAuths = append(probeAuths, auth)
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"probe boom"}`))
			return
		}
		// 聊天路径：key1 401（判停）；key2 不会被选中（探活失败后无人把它拉回 active）。
		mu.Lock()
		chatAuths = append(chatAuths, auth)
		mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	provider := models.Provider{Name: "probe-fail-loop", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	probedRecently := time.Now().Add(-30 * time.Second)
	key1 := models.Credential{
		GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key1"), KeyHash: cipher.Hash("sk-key1"),
		Status: models.CredentialStatusActive, LastProbeAt: &probedRecently,
	}
	// key2：唯一探活候选（key1 频控内被跳过），探活失败后保持 temp_unsched。
	key2 := models.Credential{
		GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key2"), KeyHash: cipher.Hash("sk-key2"),
		Status: models.CredentialStatusTempUnsched, CooldownReason: "auth_fail", FailCount: 3,
	}
	for _, cred := range []*models.Credential{&key1, &key2} {
		if err := models.DB.Create(cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
	}
	mwp := models.ModelWithProvider{ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	pool := CandidatePool{
		ProviderMap: map[uint]models.Provider{
			provider.ID: provider,
		},
		ModelWithProviderMap: map[uint]models.ModelWithProvider{
			mwp.ID: mwp,
		},
		WeightItems:   map[uint]int{mwp.ID: 10},
		PriorityItems: map[uint]int{mwp.ID: 1},
	}

	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now(),
		Style:         string(consts.StyleOpenAI),
		Before:        Before{Model: "m", Stream: false, raw: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName: "m",
		Pool:          pool,
		MaxRetry:      5,
		ClientTimeout: 2 * time.Second,
		Deadline:      make(chan time.Time),
		DeadlineErr:   errors.New("unused"),
	})

	if outcome.Status != retryLoopExhausted {
		t.Fatalf("status = %d, want %d; err = %v", outcome.Status, retryLoopExhausted, outcome.Err)
	}

	mu.Lock()
	if len(chatAuths) != 1 || chatAuths[0] != "Bearer sk-key1" {
		mu.Unlock()
		t.Fatalf("chat auth sequence = %v, want [Bearer sk-key1]（key1 401 后探活失败组耗尽，不再有第二次聊天尝试）", chatAuths)
	}
	if len(probeAuths) != 1 || probeAuths[0] != "Bearer sk-key2" {
		mu.Unlock()
		t.Fatalf("probe auths = %v, want [Bearer sk-key2]（耗尽只探 key2 一次）", probeAuths)
	}
	mu.Unlock()

	// 组织级处置在失败结局真实可观测（无成功出口重置）：候选被移除（401 语义
	// RemoveWeight+RemovePriority），ConsecutiveFailures 补累计一次。
	if _, ok := pool.WeightItems[mwp.ID]; ok {
		t.Fatalf("WeightItems[%d] still present, want removed（内层耗尽未恢复 → 组织级 RemoveWeight）", mwp.ID)
	}
	var mwpReload models.ModelWithProvider
	if err := models.DB.First(&mwpReload, mwp.ID).Error; err != nil {
		t.Fatalf("reload mwp: %v", err)
	}
	if mwpReload.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1（内层耗尽且探活未恢复 → 层内耗尽补累计一次）", mwpReload.ConsecutiveFailures)
	}

	// key2 探活失败：仅 touch——LastProbeAt 被 stamp（claim/touch 都会写，锁「探活确曾
	// 发生」），status 保持 temp_unsched 不被拉回 active（恢复失败不升级、不覆盖终态）。
	var key2Reload models.Credential
	if err := models.DB.First(&key2Reload, key2.ID).Error; err != nil {
		t.Fatalf("reload key2: %v", err)
	}
	if key2Reload.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("key2 Status = %q, want temp_unsched（探活失败不恢复）", key2Reload.Status)
	}
	if key2Reload.LastProbeAt == nil {
		t.Fatalf("key2 LastProbeAt = nil, want 非 nil（探活已 stamp，本次失败仅 touch 更新下次可探时间）")
	}
}
