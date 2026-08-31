package chat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
)

// TestProbeRecoverInGroup_RestoresAndSelectable 锁 AC-1 端到端：
// 组内仅 temp_unsched → Select 失败带回 Endpoint/Group → 探活成功 → 刷新后可选中。
func TestProbeRecoverInGroup_RestoresAndSelectable(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	SetSettingsReader(credCooldownSettingsReader{
		Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 3, ProbeIntervalSec: 60,
	})
	t.Cleanup(func() { SetSettingsReader(nil) })

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"m1","object":"model"}]}`))
	}))
	t.Cleanup(srv.Close)

	provider := models.Provider{
		Name:   "probe-provider",
		Type:   "openai",
		Config: `{"base_url":"https://unused.example/v1"}`,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{
		ProviderID: provider.ID,
		Protocol:   string(consts.ProtocolOpenAI),
		URL:        srv.URL,
		Enabled:    true,
	}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	plain := "sk-recover"
	cred := models.Credential{
		GroupID:        &grp.ID,
		Key:            mustEncrypt(t, cipher, plain),
		KeyHash:        cipher.Hash(plain),
		Status:         models.CredentialStatusTempUnsched,
		CooldownReason: "auth_fail",
		FailCount:      3,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	snapshot, err := channel.NewAssembler(repos()).Assemble(context.Background(), provider)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	selection, err := channel.DefaultSelector().Select(snapshot, consts.FormatOpenAIChat, "gpt-test", time.Now())
	if err == nil {
		t.Fatal("Select succeeded before probe, want ErrNoCredentialAvailable")
	}
	if !errors.Is(err, channel.ErrNoCredentialAvailable) {
		t.Fatalf("err = %v, want ErrNoCredentialAvailable", err)
	}
	if selection.Group.ID != grp.ID {
		t.Fatalf("partial Group.ID = %d, want %d（凭据失败仍带回已选组）", selection.Group.ID, grp.ID)
	}

	if !probeRecoverInGroup(context.Background(), provider, selection.Endpoint, snapshot.CredentialsByGroup[selection.Group.ID]) {
		t.Fatal("probeRecoverInGroup = false, want true")
	}
	if hits.Load() != 1 {
		t.Fatalf("probe hits = %d, want 1", hits.Load())
	}

	refreshed, err := channel.NewAssembler(repos()).Assemble(context.Background(), provider)
	if err != nil {
		t.Fatalf("re-assemble: %v", err)
	}
	sel, err := channel.DefaultSelector().Select(refreshed, consts.FormatOpenAIChat, "gpt-test", time.Now())
	if err != nil {
		t.Fatalf("Select after probe: %v", err)
	}
	if sel.Credential.ID != cred.ID {
		t.Fatalf("selected credential %d, want %d", sel.Credential.ID, cred.ID)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusActive || got.FailCount != 0 || got.CooldownReason != "" {
		t.Fatalf("recovered fields = status=%q fail=%d reason=%q", got.Status, got.FailCount, got.CooldownReason)
	}
}

// TestProbeRecoverInGroup_LocksExhaustedGroup 锁 Critical 修复：多同权重组时，
// 探活必须用 Select 失败带回的那一组，不能再 SelectGroup（会二次推进 RR）。
func TestProbeRecoverInGroup_LocksExhaustedGroup(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	SetSettingsReader(credCooldownSettingsReader{ProbeIntervalSec: 60})
	t.Cleanup(func() { SetSettingsReader(nil) })

	var hits atomic.Int32
	var probedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		probedKey = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	t.Cleanup(srv.Close)

	provider := models.Provider{
		Name:   "multi-group",
		Type:   "openai",
		Config: `{"base_url":"https://unused.example/v1"}`,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	g1 := models.KeyGroup{ProviderID: provider.ID, Name: "exhausted", Weight: 1}
	g2 := models.KeyGroup{ProviderID: provider.ID, Name: "healthy", Weight: 1}
	if err := models.DB.Create(&g1).Error; err != nil {
		t.Fatalf("create g1: %v", err)
	}
	if err := models.DB.Create(&g2).Error; err != nil {
		t.Fatalf("create g2: %v", err)
	}
	// g1 仅 temp_unsched；g2 有 active——若二次 SelectGroup 切到 g2，探活会跳过（无 temp_unsched）
	c1 := models.Credential{
		GroupID: &g1.ID, Key: mustEncrypt(t, cipher, "sk-g1"), KeyHash: cipher.Hash("sk-g1"),
		Status: models.CredentialStatusTempUnsched, CooldownReason: "auth_fail", FailCount: 3,
	}
	c2 := models.Credential{
		GroupID: &g2.ID, Key: mustEncrypt(t, cipher, "sk-g2"), KeyHash: cipher.Hash("sk-g2"),
		Status: models.CredentialStatusActive,
	}
	if err := models.DB.Create(&c1).Error; err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := models.DB.Create(&c2).Error; err != nil {
		t.Fatalf("create c2: %v", err)
	}

	snapshot, err := channel.NewAssembler(repos()).Assemble(context.Background(), provider)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// 推进 RR 使首次 Select 命中 g1（同权重档内按装配 ID ASC：g1 先于 g2，指针 0 → g1）
	selection, err := channel.DefaultSelector().Select(snapshot, consts.FormatOpenAIChat, "m", time.Now())
	if err == nil {
		t.Fatal("expected Select to fail on exhausted g1, got success")
	}
	if selection.Group.ID != g1.ID {
		t.Fatalf("exhausted group = %d (%s), want g1=%d（测试前置：Select 应落在耗尽组）",
			selection.Group.ID, selection.Group.Name, g1.ID)
	}

	if !probeRecoverInGroup(context.Background(), provider, selection.Endpoint, snapshot.CredentialsByGroup[selection.Group.ID]) {
		t.Fatal("probe on locked g1 failed, want recover sk-g1")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
	if probedKey != "Bearer sk-g1" {
		t.Fatalf("Authorization = %q, want Bearer sk-g1（必须探耗尽组而非二次 SelectGroup 切到的组）", probedKey)
	}

	var got models.Credential
	if err := models.DB.First(&got, c1.ID).Error; err != nil {
		t.Fatalf("reload c1: %v", err)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("c1 Status = %q, want active", got.Status)
	}
}

// TestProbeRecoverInGroup_RateLimited 锁 AC-2：间隔内二次探活不打上游。
func TestProbeRecoverInGroup_RateLimited(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	SetSettingsReader(credCooldownSettingsReader{ProbeIntervalSec: 120})
	t.Cleanup(func() { SetSettingsReader(nil) })

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	t.Cleanup(srv.Close)

	recent := time.Now().Add(-30 * time.Second)
	plain := "sk-rl"
	cred := models.Credential{
		Key:            mustEncrypt(t, cipher, plain),
		KeyHash:        cipher.Hash(plain),
		Status:         models.CredentialStatusTempUnsched,
		CooldownReason: "auth_fail",
		FailCount:      3,
		LastProbeAt:    &recent,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	ok := probeRecoverInGroup(context.Background(),
		models.Provider{Name: "p", Type: "openai"},
		models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		[]models.Credential{cred},
	)
	if ok {
		t.Fatal("probeRecoverInGroup = true, want false（频控）")
	}
	if hits.Load() != 0 {
		t.Fatalf("hits = %d, want 0", hits.Load())
	}
}
