package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// 本文件锁 #21 的契约：健康检测打上游必须走「主协议端点 + 分组凭据明文」这条
// 确定性诊断链路（channel 装配），而**不是** Provider.Config。S6 起 Config 不再
// 承载 api_key，直用 Config 会让号池/内联凭据的供应商恒被上游判 401，进而经
// handleCheckResult 把完全可用的关联自动禁用。

const testCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func setupHealthCheckCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

func mustEncrypt(t *testing.T, cipher *credentialcrypto.Cipher, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt %s: %v", plain, err)
	}
	return enc
}

// chatHandlerByAuth 假上游：按凭据头（OpenAI 系 Authorization / anthropic x-api-key）
// 放行，未登记的一律 401——一把同时钉住「用了哪条凭据」「打到哪个 URL」「打了几次」。
// 姿势同 service/modelsync/upstream_test.go 的 modelsListHandlerByAuth。
func chatHandlerByAuth(allowed map[string]struct{}, hits *atomic.Int64, captured *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		if captured != nil {
			captured.Store(r.Header.Clone())
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			auth = r.Header.Get("x-api-key")
		}
		if _, ok := allowed[auth]; !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"chat.completion","model":"m","choices":[]}`))
	}
}

func seedEndpoint(t *testing.T, providerID uint, protocol, url string) models.Endpoint {
	t.Helper()
	ep := models.Endpoint{ProviderID: providerID, Protocol: protocol, URL: url, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	return ep
}

// seedPoolGroup 建「号池 + 引用该池的分组」——凭据只存在于池内（内联凭据概念已删除：
// 分组一律经 KeyGroup.PoolID 取池内凭据）。池名由组名派生，同一测试内多组不撞名。
// plainKeys 为空则建池内无凭据的组（模拟「组内全空」）。
func seedPoolGroup(t *testing.T, cipher *credentialcrypto.Cipher, providerID uint, groupName string, plainKeys ...string) (models.Pool, models.KeyGroup) {
	t.Helper()
	pool := models.Pool{Name: "pool-" + groupName}
	if err := models.DB.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	g := models.KeyGroup{ProviderID: providerID, Name: groupName, Weight: 1, PoolID: &pool.ID}
	if err := models.DB.Create(&g).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, k := range plainKeys {
		seedCredential(t, cipher, pool.ID, k)
	}
	return pool, g
}

func seedCredential(t *testing.T, cipher *credentialcrypto.Cipher, poolID uint, plain string) models.Credential {
	t.Helper()
	return seedCredentialStatus(t, cipher, poolID, plain, models.CredentialStatusActive)
}

func seedCredentialStatus(t *testing.T, cipher *credentialcrypto.Cipher, poolID uint, plain, status string) models.Credential {
	t.Helper()
	cred := models.Credential{
		PoolID:  poolID,
		Key:     mustEncrypt(t, cipher, plain),
		KeyHash: cipher.Hash(plain),
		Status:  status,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	return cred
}

func loadCapturedHeader(t *testing.T, captured *atomic.Value) http.Header {
	t.Helper()
	h, ok := captured.Load().(http.Header)
	if !ok {
		t.Fatal("上游未收到任何请求（端点 URL 或凭据装配有误）")
	}
	return h
}

// TestDoCheck_UsesEndpointURLAndPoolCredential 核心回归：Config 无 api_key 且 base_url
// 刻意指向不可达主机时，检测必须用「端点 URL + 号池凭据明文」打上游并判成功。
// 修复前直用 Provider.Config 组装 → 请求压根到不了假上游（hits=0）且判失败。
func TestDoCheck_UsesEndpointURLAndPoolCredential(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-pool-1": {}}, &hits, &captured))
	t.Cleanup(srv.Close)

	provider := models.Provider{
		Name:   "p-pool",
		Type:   "openai",
		Config: `{"base_url":"https://should-not-use.example"}`,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "池组", "sk-pool-1")

	if err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"}); err != nil {
		t.Fatalf("doCheck: %v（号池凭据未被使用即为 #21 缺陷）", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（端点 URL 必须被使用）", hits.Load())
	}
	if got := loadCapturedHeader(t, &captured).Get("Authorization"); got != "Bearer sk-pool-1" {
		t.Fatalf("Authorization = %q, want Bearer sk-pool-1", got)
	}
}

// TestDoCheck_AnthropicEndpointUsesAPIKeyHeader 锁「按端点协议派生方言」：anthropic
// 端点必须以 x-api-key 出站，用 Authorization 传 key 同样表现为恒 401。
func TestDoCheck_AnthropicEndpointUsesAPIKeyHeader(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"sk-anthropic": {}}, &hits, &captured))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-claude", Type: "anthropic", Config: `{"base_url":"https://should-not-use.example"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "anthropic", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "g1", "sk-anthropic")

	if err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "claude-sonnet-4-5"}); err != nil {
		t.Fatalf("doCheck: %v", err)
	}
	h := loadCapturedHeader(t, &captured)
	if h.Get("x-api-key") != "sk-anthropic" {
		t.Fatalf("x-api-key = %q, want sk-anthropic", h.Get("x-api-key"))
	}
	if h.Get("Authorization") != "" {
		t.Fatalf("anthropic 端点不应带 Authorization: %q", h.Get("Authorization"))
	}
}

// TestDoCheck_FallsBackToNextGroupOnUpstreamFailure 首个分组的 key 已失效时必须换下一
// 组再试，而不是就此判这条关联不健康：健康检测失败会经 handleCheckResult 自动禁用关联，
// 假失败（false negative）的代价远高于一次额外的上游请求（失败请求不产生 token 费用）。
func TestDoCheck_FallsBackToNextGroupOnUpstreamFailure(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-good": {}}, &hits, &captured))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-two", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "坏key组", "sk-dead")
	_, _ = seedPoolGroup(t, cipher, provider.ID, "好key组", "sk-good")

	if err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"}); err != nil {
		t.Fatalf("doCheck: %v（首个分组失败后应继续尝试其余分组）", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("upstream hits = %d, want 2（先坏组后好组）", hits.Load())
	}
	if got := loadCapturedHeader(t, &captured).Get("Authorization"); got != "Bearer sk-good" {
		t.Fatalf("最后一次 Authorization = %q, want Bearer sk-good", got)
	}
}

// TestDoCheck_SkipsGroupWithoutUsableCredential 组内凭据全不可用（停用）时不打上游、
// 直接换下一组——与模型同步「Skipped 不阻断其余组」同一分层语义。
func TestDoCheck_SkipsGroupWithoutUsableCredential(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-live": {}}, &hits, &captured))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-skip", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	deadPool, _ := seedPoolGroup(t, cipher, provider.ID, "停用组")
	seedCredentialStatus(t, cipher, deadPool.ID, "sk-disabled", models.CredentialStatusDisabled)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "可用组", "sk-live")

	if err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"}); err != nil {
		t.Fatalf("doCheck: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（无可用凭据的组不得打上游）", hits.Load())
	}
	if got := loadCapturedHeader(t, &captured).Get("Authorization"); got != "Bearer sk-live" {
		t.Fatalf("Authorization = %q, want Bearer sk-live", got)
	}
}

// TestDoCheck_NoUsableCredentialFailsWithoutHittingUpstream 全供应商无可用凭据 → 明确
// 失败且不打上游（错误文案带分组名，供 health_check_logs 直接可读）。
func TestDoCheck_NoUsableCredentialFailsWithoutHittingUpstream(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-nokey", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	offPool, _ := seedPoolGroup(t, cipher, provider.ID, "全停用组")
	seedCredentialStatus(t, cipher, offPool.ID, "sk-off", models.CredentialStatusDisabled)

	err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"})
	if err == nil {
		t.Fatal("want error when no credential is usable")
	}
	if !strings.Contains(err.Error(), "全停用组") {
		t.Fatalf("err = %v, want 含分组名", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestDoCheck_UpstreamFailureSurfacesStatusCode 所有分组都被上游拒绝 → 失败且错误里留得
// 下上游状态码（health_check_logs.error 是排查入口，不能只剩「failed」）。
func TestDoCheck_UpstreamFailureSurfacesStatusCode(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-bad", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "g1", "sk-invalid")

	err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"})
	if err == nil {
		t.Fatal("want error when upstream rejects every group")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want 含上游状态码 401", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
}

// TestDoCheck_NoPrimaryEndpoint 主协议端点缺失/停用 → 显式失败，不静默回落到
// Provider.Config 的 base_url（回落正是本次要消灭的旁路）。
func TestDoCheck_NoPrimaryEndpoint(t *testing.T) {
	initHealthCheckTestDB(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-1": {}}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-noep", Type: "openai", Config: `{"base_url":"https://should-not-use.example"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	// 只有 anthropic 端点：openai 主协议端点缺失
	seedEndpoint(t, provider.ID, "anthropic", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "g1", "sk-1")

	err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"})
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("err = %v, want 主协议端点缺失语义", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestDoCheck_NoKeyGroups 无任何分组 → 明确失败（分组是 S6 后唯一的凭据来源）。
func TestDoCheck_NoKeyGroups(t *testing.T) {
	initHealthCheckTestDB(t)
	_ = setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-nogroups", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)

	err := (&HealthChecker{}).doCheck(context.Background(), &provider, &models.ModelWithProvider{ProviderModel: "gpt-4.1"})
	if err == nil || !strings.Contains(err.Error(), "key group") {
		t.Fatalf("err = %v, want 无分组语义", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestCheckSingleWithBatch_PoolCredentialLogsSuccess 端到端锁真实后果：手动检测一条
// 号池供应商的关联必须写 success 日志、且不得把关联禁用（修复前每轮都记 401，
// 三个周期即触发自动禁用）。
func TestCheckSingleWithBatch_PoolCredentialLogsSuccess(t *testing.T) {
	// newTestChecker 内含 initHealthCheckTestDB：必须先拿 checker 再种数据，
	// 否则种子会落在被后一次 Init 顶掉的连接上。
	h, mgr := newTestChecker(t)
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-e2e": {}}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-e2e", Type: "openai", Config: `{"base_url":"https://should-not-use.example"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "e2e组", "sk-e2e")

	model := models.Model{Name: "gpt-4.1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	enabled := true
	mp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "gpt-4.1", Weight: 10, Priority: 10, Status: &enabled}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	logEntry, err := h.CheckSingleWithBatch(context.Background(), mp.ID, "b1")
	if err != nil {
		t.Fatalf("CheckSingleWithBatch: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
	if logEntry.Status != "success" {
		t.Fatalf("log status = %q (%s), want success", logEntry.Status, logEntry.Error)
	}

	got, err := repository.Default().ModelWithProvider.Get(context.Background(), mp.ID)
	if err != nil {
		t.Fatalf("reload association: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Fatalf("association disabled after health check: status=%v", got.Status)
	}
	waitDrained(t, mgr)
}
