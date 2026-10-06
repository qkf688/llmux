package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// setupAffinityFixture 装配「1 供应商 / 1 分组 / 2 凭据 / 1 关联」的最小粘性装置：
// 上游记录每请求的 Authorization，凭据创建顺序 = 轮询顺序（key1 先现）。
// 返回 runReq（带对话体发一次请求）、auths 快照访问器与 key1 记录（供冷却注入）。
func setupAffinityFixture(t *testing.T) (runReq func(raw string), auths func() []string, key1 models.Credential) {
	t.Helper()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	balancer.ResetDefaultAffinityForTest()
	t.Cleanup(balancer.ResetDefaultAffinityForTest)

	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatSuccessBody))
	}))
	t.Cleanup(srv.Close)

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "aff-p", Type: providers.TypeOpenAI, Config: `{"base_url":"` + srv.URL + `"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	pool := models.Pool{Name: "pool-1"}
	if err := models.DB.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1, PoolID: &pool.ID}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	for i, plain := range []string{"sk-key1", "sk-key2"} {
		cred := models.Credential{PoolID: pool.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain)}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential %s: %v", plain, err)
		}
		if i == 0 {
			key1 = cred
		}
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	runReq = func(raw string) {
		t.Helper()
		before, err := BeforerOpenAI([]byte(raw))
		if err != nil {
			t.Fatalf("BeforerOpenAI: %v", err)
		}
		pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
		if err != nil {
			t.Fatalf("build candidate pool: %v", err)
		}
		if _, err := BalanceChat(context.Background(), BalanceInput{
			Start:  time.Now(),
			Style:  string(consts.StyleOpenAI),
			Before: *before,
			ProvidersWithMeta: ProvidersWithMeta{
				CandidatePool: pool,
				Model:         &models.Model{Name: "m1"},
			},
			ReqMeta: models.ReqMeta{Header: http.Header{}},
		}); err != nil {
			t.Fatalf("BalanceChat err = %v", err)
		}
	}
	auths = func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
	return runReq, auths, key1
}

// TestBalanceChat_CacheAffinity_PinsSameConversation 锁 AC-1：同一对话身份的多轮请求
// 钉住同一凭据（第 2 轮追加消息后仍命中同 key——无粘性时轮询必已推进）；不同对话
// 互不钉住（按现状轮询落到另一把 key）。
func TestBalanceChat_CacheAffinity_PinsSameConversation(t *testing.T) {
	runReq, auths, _ := setupAffinityFixture(t)

	// 对话 A 第 1 轮：轮询首现 key1，成功后钉定
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-A"},{"role":"user","content":"hi-A"}]}`)
	// 对话 A 第 2 轮：头部语义不变、尾部追加 → 同键，钉回 key1
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-A"},{"role":"user","content":"hi-A"},{"role":"assistant","content":"hello"},{"role":"user","content":"more-A"}]}`)
	// 对话 B：system 与首条消息都不同 → 不与 A 互钉，轮询推进到 key2
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-B"},{"role":"user","content":"hi-B"}]}`)

	want := []string{"Bearer sk-key1", "Bearer sk-key1", "Bearer sk-key2"}
	got := auths()
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("auth 序列 = %v, want %v（同对话钉同 key；异对话轮询推进不互钉）", got, want)
	}
}

// TestBalanceChat_CacheAffinity_CoolingFailsOverAndRepins 锁 AC-2：钉定 key 被冷却后
// 静默回退轮询选中 key2 且请求成功；成功后**改钉** key2（第 3 次请求钉 key2 而非
// 回落 key1）——转移成功改钉，粘性不把对话锁死在失效目标上。
func TestBalanceChat_CacheAffinity_CoolingFailsOverAndRepins(t *testing.T) {
	runReq, auths, key1 := setupAffinityFixture(t)

	// 对话 C 第 1 轮：命中并钉定 key1
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-C"},{"role":"user","content":"hi-C"}]}`)

	// 把 key1 打冷却（模拟上一轮凭据级失败写库后的状态）
	coolUntil := time.Now().Add(10 * time.Minute)
	if err := models.DB.Model(&models.Credential{}).
		Where("id = ?", key1.ID).
		Updates(map[string]any{"cooldown_until": coolUntil}).Error; err != nil {
		t.Fatalf("cool key1: %v", err)
	}

	// 对话 C 第 2 轮：钉定的 key1 冷却 → 回退轮询 key2 成功 → 改钉 key2
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-C"},{"role":"user","content":"hi-C"},{"role":"assistant","content":"hello"},{"role":"user","content":"more-C"}]}`)

	// key1 冷却到期（恢复可选）——让「改钉」与「碰巧轮询到」可区分：
	// 若第 2 轮成功后未改钉 key2，第 3 轮的轮询必回到 key1。
	if err := models.DB.Model(&models.Credential{}).
		Where("id = ?", key1.ID).
		Updates(map[string]any{"cooldown_until": nil}).Error; err != nil {
		t.Fatalf("expire key1 cooldown: %v", err)
	}

	// 对话 C 第 3 轮：钉定已改到 key2（未改钉的话轮询会回 key1）
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-C"},{"role":"user","content":"hi-C"},{"role":"assistant","content":"hello"},{"role":"user","content":"more-C"},{"role":"assistant","content":"ok"},{"role":"user","content":"again"}]}`)

	want := []string{"Bearer sk-key1", "Bearer sk-key2", "Bearer sk-key2"}
	got := auths()
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("auth 序列 = %v, want %v（冷却回退 + 成功改钉）", got, want)
	}
}

// TestBalanceChat_CacheAffinity_DisabledKeepsRoundRobin 锁 AC-5：开关关闭 = 与现状
// 完全一致——同一对话两次请求仍按轮询逐请求推进，不发生钉定。
func TestBalanceChat_CacheAffinity_DisabledKeepsRoundRobin(t *testing.T) {
	runReq, auths, _ := setupAffinityFixture(t)
	SetSettingsReader(affinityGateStub{enabled: false})
	t.Cleanup(func() { SetSettingsReader(nil) })

	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-D"},{"role":"user","content":"hi-D"}]}`)
	runReq(`{"model":"m1","messages":[{"role":"system","content":"sys-D"},{"role":"user","content":"hi-D"},{"role":"assistant","content":"hello"},{"role":"user","content":"more-D"}]}`)

	want := []string{"Bearer sk-key1", "Bearer sk-key2"}
	got := auths()
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("auth 序列 = %v, want %v（关闭开关必须逐请求轮询，不钉）", got, want)
	}
}

// TestBalanceChatVirtual_CacheAffinity_PinsRealModelFirst 锁 AC-4：钉定的真实模型
// 排尝试首位（本地副本稳定前移，其余顺序不变，不动 virtualmodel 策略状态）；未钉时
// 按给定顺序逐个尝试。
func TestBalanceChatVirtual_CacheAffinity_PinsRealModelFirst(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	balancer.ResetDefaultAffinityForTest()
	t.Cleanup(balancer.ResetDefaultAffinityForTest)

	var mu sync.Mutex
	var hit1, hit2 int
	mkSrv := func(counter *int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			*counter++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(chatSuccessBody))
		}))
	}
	srv1, srv2 := mkSrv(&hit1), mkSrv(&hit2)
	t.Cleanup(srv1.Close)
	t.Cleanup(srv2.Close)

	enabled := true
	first := models.Model{Name: "real-first"}
	second := models.Model{Name: "real-second"}
	for _, m := range []*models.Model{&first, &second} {
		if err := models.DB.Create(m).Error; err != nil {
			t.Fatalf("create real model %q: %v", m.Name, err)
		}
	}
	mkReal := func(m models.Model, srv *httptest.Server, name, plain string) (models.ModelWithProvider, models.KeyGroup, models.Credential) {
		t.Helper()
		provider := models.Provider{Name: name, Type: providers.TypeOpenAI, Config: `{"base_url":"` + srv.URL + `"}`}
		if err := models.DB.Create(&provider).Error; err != nil {
			t.Fatalf("create provider %s: %v", name, err)
		}
		ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
		if err := models.DB.Create(&ep).Error; err != nil {
			t.Fatalf("create endpoint %s: %v", name, err)
		}
		pool := models.Pool{Name: "pool-" + name}
		if err := models.DB.Create(&pool).Error; err != nil {
			t.Fatalf("create pool %s: %v", name, err)
		}
		grp := models.KeyGroup{ProviderID: provider.ID, Name: name + "-g", Weight: 1, PoolID: &pool.ID}
		if err := models.DB.Create(&grp).Error; err != nil {
			t.Fatalf("create group %s: %v", name, err)
		}
		cred := models.Credential{PoolID: pool.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain)}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential %s: %v", name, err)
		}
		mwp := models.ModelWithProvider{ModelID: m.ID, ProviderID: provider.ID, ProviderModel: name + "-pm", Status: &enabled, Weight: 10, Priority: 1}
		if err := models.DB.Create(&mwp).Error; err != nil {
			t.Fatalf("create association %s: %v", name, err)
		}
		return mwp, grp, cred
	}
	mkReal(first, srv1, "p1", "sk-p1")
	mwp2, grp2, cred2 := mkReal(second, srv2, "p2", "sk-p2")

	runVirtual := func(before Before) error {
		_, err := balanceChatVirtual(context.Background(), BalanceInput{
			Start:  time.Now(),
			Style:  string(consts.StyleOpenAI),
			Before: before,
			ProvidersWithMeta: ProvidersWithMeta{
				IsVirtualModel:    true,
				VirtualModelName:  "vm-pin",
				VirtualStrategy:   "priority",
				OrderedRealModels: []virtualmodel.OrderedRealModel{{Model: first}, {Model: second}},
			},
			ReqMeta: models.ReqMeta{Header: http.Header{}},
		})
		return err
	}

	// ① 钉定 second 的完整目标（模拟上一次在 second 上成功）→ 直接打 srv2，srv1 不被碰
	pinned, err := BeforerOpenAI([]byte(`{"model":"vm-pin","messages":[{"role":"system","content":"sys-pin"},{"role":"user","content":"hi-pin"}]}`))
	if err != nil {
		t.Fatalf("BeforerOpenAI: %v", err)
	}
	balancer.DefaultAffinity().Put(pinned.affinityKey, balancer.Target{
		ModelID:             second.ID,
		ModelWithProviderID: mwp2.ID,
		GroupID:             grp2.ID,
		CredentialID:        cred2.ID,
	}, time.Now())
	if err := runVirtual(*pinned); err != nil {
		t.Fatalf("pinned run err = %v", err)
	}
	mu.Lock()
	if hit2 != 1 || hit1 != 0 {
		mu.Unlock()
		t.Fatalf("钉定后命中 = srv1:%d srv2:%d, want srv1:0 srv2:1（钉定真实模型排首位）", hit1, hit2)
	}
	mu.Unlock()

	// ② 未钉对话 → 按给定顺序先试 first → srv1
	fresh, err := BeforerOpenAI([]byte(`{"model":"vm-pin","messages":[{"role":"system","content":"sys-free"},{"role":"user","content":"hi-free"}]}`))
	if err != nil {
		t.Fatalf("BeforerOpenAI: %v", err)
	}
	if err := runVirtual(*fresh); err != nil {
		t.Fatalf("unpinned run err = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if hit1 != 1 || hit2 != 1 {
		t.Fatalf("未钉命中 = srv1:%d srv2:%d, want srv1:1 srv2:1（未钉按给定顺序）", hit1, hit2)
	}
}
