package credprobe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/credwrite"
	"gorm.io/gorm"
)

const testCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func initCredProbeTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	repository.SetDefault(repository.New(models.DB))
	credwrite.EnableTestSyncMode(true)
	credwrite.ResetDroppedForTest()
	// 恢复生产 merge 的 probe handlers（其他包 ReplaceHandlersForTest 可能清空）。
	credwrite.SetHandlers(credwrite.Handlers{
		TouchProbe: persistTouchProbe,
		Recover:    persistRecover,
	})
	t.Cleanup(func() {
		credwrite.EnableTestSyncMode(true)
		credwrite.ResetDroppedForTest()
		repository.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func setupCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

func encryptKey(t *testing.T, cipher *credentialcrypto.Cipher, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return enc
}

func modelsOKServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/models" && r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-test","object":"model"}]}`))
	}))
}

func modelsFailServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
}

func seedTempUnschedCred(t *testing.T, cipher *credentialcrypto.Cipher, plain string, lastProbe *time.Time) models.Credential {
	t.Helper()
	cred := models.Credential{
		Key:            encryptKey(t, cipher, plain),
		KeyHash:        cipher.Hash(plain),
		Status:         models.CredentialStatusTempUnsched,
		CooldownReason: "auth_fail",
		FailCount:      3,
		LastProbeAt:    lastProbe,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	return cred
}

// TestTryRecover_SuccessRestoresActive 锁 AC-1/AC-6：探活成功 → active + 清计数/reason + LastProbeAt。
func TestTryRecover_SuccessRestoresActive(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsOKServer(t, &hits)
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-probe-ok", nil)
	cred2 := seedTempUnschedCred(t, cipher, "sk-probe-ok-2", nil) // 多候选只探 1 条

	now := time.Now()
	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai", Config: `{"base_url":"https://unused.example/v1"}`},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred2, cred}, // 故意乱序；应选 ID 更小的
		Interval:    60 * time.Second,
		Now:         now,
		Timeout:     2 * time.Second,
	})
	if !out.Attempted || !out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted+Recovered", out)
	}
	if out.CredentialID != cred.ID {
		t.Fatalf("CredentialID = %d, want smaller ID %d（ID ASC）", out.CredentialID, cred.ID)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（本轮最多探 1 条）", hits.Load())
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active", got.Status)
	}
	if got.FailCount != 0 {
		t.Fatalf("FailCount = %d, want 0（CredentialRecoveryFields）", got.FailCount)
	}
	if got.CooldownReason != "" {
		t.Fatalf("CooldownReason = %q, want empty", got.CooldownReason)
	}
	if got.LastProbeAt == nil {
		t.Fatal("LastProbeAt = nil, want set after probe")
	}

	var other models.Credential
	if err := models.DB.First(&other, cred2.ID).Error; err != nil {
		t.Fatalf("reload other: %v", err)
	}
	if other.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("second cred Status = %q, want still temp_unsched（只探 1 条）", other.Status)
	}
}

// TestTryRecover_RateLimitSkips 锁 AC-2：间隔内不重探。
func TestTryRecover_RateLimitSkips(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsOKServer(t, &hits)
	t.Cleanup(srv.Close)

	recent := time.Now().Add(-10 * time.Second)
	cred := seedTempUnschedCred(t, cipher, "sk-rate", &recent)

	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         time.Now(),
	})
	if out.Attempted || out.Recovered {
		t.Fatalf("outcome = %+v, want skip（间隔内）", out)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestTryRecover_ProbeFailureOnlyTouchesLastProbeAt 锁 AC-3：探失败只更新时间。
func TestTryRecover_ProbeFailureOnlyTouchesLastProbeAt(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsFailServer(t, &hits)
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-fail", nil)
	now := time.Now()
	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         now,
	})
	if !out.Attempted || out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted without Recovered", out)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("Status = %q, want temp_unsched", got.Status)
	}
	if got.FailCount != 3 || got.CooldownReason != "auth_fail" {
		t.Fatalf("FailCount/Reason changed: %d %q", got.FailCount, got.CooldownReason)
	}
	if got.LastProbeAt == nil {
		t.Fatal("LastProbeAt = nil after failed probe")
	}
}

// TestTryRecover_DoesNotOverwriteManualTerminal 锁 AC-4：写回前已变 disabled 不恢复
// （条件更新 WHERE status=temp_unsched，RowsAffected==0）。
func TestTryRecover_DoesNotOverwriteManualTerminal(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32

	cred := seedTempUnschedCred(t, cipher, "sk-manual", nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if err := models.DB.Model(&models.Credential{}).Where("id = ?", cred.ID).
			Update("status", models.CredentialStatusDisabled).Error; err != nil {
			t.Errorf("flip to disabled: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"object":"list","data":[]}`)
	}))
	t.Cleanup(srv.Close)

	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         time.Now(),
	})
	if !out.Attempted || out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted without Recovered（人工终态保护）", out)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusDisabled {
		t.Fatalf("Status = %q, want disabled（不得拉回 active）", got.Status)
	}
	if got.LastProbeAt == nil {
		t.Fatal("LastProbeAt should still be updated")
	}
}

// TestTryRecover_StripsCustomModelsHitsUpstream 锁 Warning 修复：config 含 custom_models
// 时仍必须打上游 /models（否则 Models() 本地短路 → 假成功拉回失效 key）。
func TestTryRecover_StripsCustomModelsHitsUpstream(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsOKServer(t, &hits)
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-custom", nil)
	out := TryRecover(Request{
		Ctx: context.Background(),
		Provider: models.Provider{
			Name:   "p",
			Type:   "openai",
			Config: `{"base_url":"https://unused.example/v1","custom_models":["local-only"]}`,
		},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         time.Now(),
	})
	if !out.Attempted || !out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted+Recovered", out)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（custom_models 必须被剥掉）", hits.Load())
	}
}

// TestPickProbeCandidate_SkipsNonTempUnsched 锁候选过滤：active/error/disabled 不探。
func TestPickProbeCandidate_SkipsNonTempUnsched(t *testing.T) {
	now := time.Now()
	creds := []models.Credential{
		{Model: gorm.Model{ID: 1}, Status: models.CredentialStatusActive},
		{Model: gorm.Model{ID: 2}, Status: models.CredentialStatusError},
		{Model: gorm.Model{ID: 3}, Status: models.CredentialStatusDisabled},
	}
	if got := pickProbeCandidate(creds, now, time.Minute); got != nil {
		t.Fatalf("picked %+v, want nil", got)
	}
}

// TestStripCustomModels 锁剥字段纯逻辑。
func TestStripCustomModels(t *testing.T) {
	got, err := stripCustomModels(`{"api_key":"k","custom_models":["a"],"base_url":"https://x"}`)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if strings.Contains(got, "custom_models") {
		t.Fatalf("still has custom_models: %s", got)
	}
	if !strings.Contains(got, `"api_key":"k"`) {
		t.Fatalf("lost api_key: %s", got)
	}
}

// TestTryRecover_ConcurrentDualRequestSingleUpstream 锁 AC-1：同凭据并发双请求最多 1 次上游。
func TestTryRecover_ConcurrentDualRequestSingleUpstream(t *testing.T) {
	initCredProbeTestDB(t)
	cipher := setupCipher(t)
	var hits atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-test","object":"model"}]}`))
	}))
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-dual", nil)
	now := time.Now()
	req := Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         now,
		Timeout:     3 * time.Second,
	}

	var wg sync.WaitGroup
	outcomes := make([]Outcome, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outcomes[i] = TryRecover(req)
		}(i)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		wg.Wait()
		t.Fatal("upstream was not hit")
	}
	// 给第二路一点时间撞上 CAS 未 claim 路径
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
	attempted := 0
	recovered := 0
	for _, o := range outcomes {
		if o.Attempted {
			attempted++
		}
		if o.Recovered {
			recovered++
		}
	}
	if attempted != 1 {
		t.Fatalf("Attempted count = %d, want 1", attempted)
	}
	if recovered != 1 {
		t.Fatalf("Recovered count = %d, want 1", recovered)
	}
}

// TestTryRecover_RecoverVisibleWithoutFlush 锁 Critical 修复：关 syncMode 时恢复仍
// 立刻可见（同步落库），本请求 Assemble/Select 不依赖 Flush。
func TestTryRecover_RecoverVisibleWithoutFlush(t *testing.T) {
	initCredProbeTestDB(t)
	credwrite.EnableTestSyncMode(false)
	t.Cleanup(func() { credwrite.EnableTestSyncMode(true) })

	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsOKServer(t, &hits)
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-sync-vis", nil)
	now := time.Now()
	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         now,
		Timeout:     2 * time.Second,
	})
	if !out.Attempted || !out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted+Recovered（无需 Flush）", out)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active before Flush", got.Status)
	}
	if got.FailCount != 0 || got.CooldownReason != "" {
		t.Fatalf("FailCount/Reason = %d %q, want cleared", got.FailCount, got.CooldownReason)
	}
}

// TestTryRecover_FailTouchAsyncEventuallyPersists 失败 touch 仍走异步队列，Flush 后可见。
func TestTryRecover_FailTouchAsyncEventuallyPersists(t *testing.T) {
	initCredProbeTestDB(t)
	credwrite.EnableTestSyncMode(false)
	credwrite.Start()
	t.Cleanup(func() { credwrite.EnableTestSyncMode(true) })

	cipher := setupCipher(t)
	var hits atomic.Int32
	srv := modelsFailServer(t, &hits)
	t.Cleanup(srv.Close)

	cred := seedTempUnschedCred(t, cipher, "sk-async-touch", nil)
	// 清掉 CAS 留下的时间，便于断言 touch 入队后的刷新（CAS 与 touch 同戳时仍非 nil）。
	now := time.Now()
	out := TryRecover(Request{
		Ctx:         context.Background(),
		Provider:    models.Provider{Name: "p", Type: "openai"},
		Endpoint:    models.Endpoint{Protocol: "openai", URL: srv.URL, Enabled: true},
		Credentials: []models.Credential{cred},
		Interval:    60 * time.Second,
		Now:         now,
	})
	if !out.Attempted || out.Recovered {
		t.Fatalf("outcome = %+v, want Attempted without Recovered", out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := credwrite.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("Status = %q, want temp_unsched", got.Status)
	}
	if got.LastProbeAt == nil {
		t.Fatal("LastProbeAt = nil after Flush（CAS 或 touch 应已写入）")
	}
}
