package modelsync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// modelsListHandlerByAuth 按 Authorization 头返回不同模型列表，用于验证「哪条凭据打了上游」
// 与多分组并集；未登记的 key 返回 401（模拟上游对缺 key/错 key 的判定）。
func modelsListHandlerByAuth(listsByAuth map[string]string, hits *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if hits != nil {
			hits.Add(1)
		}
		body, ok := listsByAuth[r.Header.Get("Authorization")]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func modelListBody(ids ...string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf(`{"id":%q,"object":"model","created":0,"owned_by":"o"}`, id))
	}
	return `{"object":"list","data":[` + strings.Join(parts, ",") + `]}`
}

// seedInlineGroup 建一个内联凭据分组（凭据 active），返回分组。plainKey 为空则建空组。
func seedInlineGroup(t *testing.T, db *gorm.DB, cipher *credentialcrypto.Cipher, providerID uint, name, plainKey string) models.KeyGroup {
	t.Helper()
	return seedInlineGroupWithStatus(t, db, cipher, providerID, name, plainKey, models.CredentialStatusActive)
}

// seedInlineGroupWithStatus 同 seedInlineGroup，但可指定凭据状态（验证非 active 凭据不可调度）。
func seedInlineGroupWithStatus(t *testing.T, db *gorm.DB, cipher *credentialcrypto.Cipher, providerID uint, name, plainKey, status string) models.KeyGroup {
	t.Helper()
	g := models.KeyGroup{ProviderID: providerID, Name: name, Weight: 1}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("create group %s: %v", name, err)
	}
	if plainKey == "" {
		return g
	}
	cred := models.Credential{
		GroupID: &g.ID,
		Key:     mustEncryptKey(t, cipher, plainKey),
		KeyHash: cipher.Hash(plainKey),
		Status:  status,
	}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	return g
}

// TestListUpstreamModels_UsesEndpointURLAndGroupCredential 锁本次修复的核心契约：
// Provider.Config 不含 api_key 且 base_url 指向错误主机时，只读浏览仍必须用
// 「主协议端点 URL + 分组凭据明文」打上游（此前直用 Config 组装 → 上游恒 401）。
// 同时验证 custom_models 被剥离（否则 Models() 本地短路，压根不发请求）与「不落库」。
func TestListUpstreamModels_UsesEndpointURLAndGroupCredential(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	ctx := context.Background()

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandlerByAuth(map[string]string{
		"Bearer sk-g1": modelListBody("m1", "m2"),
	}, &hits))
	t.Cleanup(server.Close)

	provider := models.Provider{
		Name:   "p-browse",
		Type:   "openai",
		Config: `{"base_url":"https://should-not-use.example","custom_models":["c1"]}`,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	group := seedInlineGroup(t, db, cipher, provider.ID, "g1", "sk-g1")

	got, err := ListUpstreamModels(ctx, repository.New(db), provider.ID)
	if err != nil {
		t.Fatalf("ListUpstreamModels: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
	if len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Fatalf("models = %+v, want [m1 m2]（custom_models 不得短路）", got)
	}

	// 只读：不写分组白名单、不产同步日志
	after, err := repository.New(db).KeyGroup.Get(ctx, group.ID)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if after.Models != "" {
		t.Fatalf("KeyGroup.Models = %q, want empty（只读浏览不得落库）", after.Models)
	}
	var logs int64
	if err := db.Model(&models.ModelSyncLog{}).Count(&logs).Error; err != nil {
		t.Fatalf("count sync logs: %v", err)
	}
	if logs != 0 {
		t.Fatalf("ModelSyncLog count = %d, want 0", logs)
	}
}

// TestListUpstreamModels_PoolCredential 覆盖用户实际形态：单分组关联号池，
// 凭据在池内（Credential.PoolID），必须被选中并解密成 Authorization 头。
func TestListUpstreamModels_PoolCredential(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandlerByAuth(map[string]string{
		"Bearer sk-pool-1": modelListBody("pool-m1"),
	}, &hits))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-pool", Type: "openai", Config: `{"base_url":"` + server.URL + `"}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	// 端点 URL 留空：验证继承链回落到 Config.base_url
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: "", Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	pool := models.Pool{Name: "池1"}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	group := models.KeyGroup{ProviderID: provider.ID, Name: "池组", Weight: 1, PoolID: &pool.ID}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	cred := models.Credential{
		PoolID:  &pool.ID,
		Key:     mustEncryptKey(t, cipher, "sk-pool-1"),
		KeyHash: cipher.Hash("sk-pool-1"),
		Status:  models.CredentialStatusActive,
	}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	got, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err != nil {
		t.Fatalf("ListUpstreamModels: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
	if len(got) != 1 || got[0].ID != "pool-m1" {
		t.Fatalf("models = %+v, want [pool-m1]", got)
	}
}

// TestListUpstreamModels_MergesGroupsAndDedups 多分组：逐组各打一次上游，
// 结果按模型 ID 去重合并（组织级视图 = 各分组授权集并集）。
func TestListUpstreamModels_MergesGroupsAndDedups(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandlerByAuth(map[string]string{
		"Bearer sk-g1": modelListBody("m1", "m2"),
		"Bearer sk-g2": modelListBody("m2", "m3"),
	}, &hits))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-multi", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	seedInlineGroup(t, db, cipher, provider.ID, "g1", "sk-g1")
	seedInlineGroup(t, db, cipher, provider.ID, "g2", "sk-g2")

	got, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err != nil {
		t.Fatalf("ListUpstreamModels: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("upstream hits = %d, want 2（每分组一次）", hits.Load())
	}
	ids := make([]string, 0, len(got))
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "m1,m2,m3" {
		t.Fatalf("merged ids = %v, want [m1 m2 m3]（m2 去重、分组顺序稳定）", ids)
	}
}

// TestListUpstreamModels_EmptyGroupDoesNotBlockOthers 无凭据的分组只是跳过，
// 其余分组仍贡献模型（与同步的 Skipped 语义同构）。
func TestListUpstreamModels_EmptyGroupDoesNotBlockOthers(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	server := httptest.NewServer(modelsListHandlerByAuth(map[string]string{
		"Bearer sk-g2": modelListBody("m9"),
	}, nil))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-mixed", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	seedInlineGroup(t, db, cipher, provider.ID, "空组", "")
	seedInlineGroup(t, db, cipher, provider.ID, "有key组", "sk-g2")

	got, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err != nil {
		t.Fatalf("ListUpstreamModels: %v", err)
	}
	if len(got) != 1 || got[0].ID != "m9" {
		t.Fatalf("models = %+v, want [m9]", got)
	}
}

// TestListUpstreamModels_NoUsableCredential 全供应商无可用凭据 → 明确报错，不打上游。
func TestListUpstreamModels_NoUsableCredential(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandlerByAuth(nil, &hits))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-nokey", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	seedInlineGroupWithStatus(t, db, cipher, provider.ID, "禁用key组", "sk-disabled", models.CredentialStatusDisabled)

	_, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err == nil || !strings.Contains(err.Error(), "no usable credential") {
		t.Fatalf("err = %v, want 'no usable credential'", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestListUpstreamModels_AllGroupsFailSurfacesUpstreamError 全部组都打了上游但都失败
// （例如 key 无效）→ 把上游原文透出，供管理端弹窗直接展示。
func TestListUpstreamModels_AllGroupsFailSurfacesUpstreamError(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	server := httptest.NewServer(modelsListHandlerByAuth(map[string]string{}, nil))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-bad", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	seedInlineGroup(t, db, cipher, provider.ID, "g1", "sk-invalid")

	_, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err == nil {
		t.Fatal("expected error when every group fails upstream")
	}
	if !strings.Contains(err.Error(), `group "g1"`) {
		t.Fatalf("err = %v, want group context", err)
	}
}

// TestListUpstreamModels_NoEnabledEndpoint 主协议端点缺失/停用 → 明确报错（不静默回落）。
func TestListUpstreamModels_NoEnabledEndpoint(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)

	provider := models.Provider{Name: "p-noep", Type: "openai", Config: `{"base_url":"https://x.example"}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: "https://x.example", Enabled: false}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	// Endpoint.Enabled 带 gorm:"default:true"：Create(struct) 跳过零值 false 会落成 true
	// （与 providerapi.syncEndpoints 同一坑），必须用 map 显式写 false。
	if err := db.Model(&models.Endpoint{}).Where("id = ?", ep.ID).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable endpoint: %v", err)
	}
	seedInlineGroup(t, db, cipher, provider.ID, "g1", "sk-g1")

	_, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err == nil || !strings.Contains(err.Error(), "no enabled endpoint") {
		t.Fatalf("err = %v, want 'no enabled endpoint'", err)
	}
}

// TestListUpstreamModels_NoGroups 无分组 → 明确报错。
func TestListUpstreamModels_NoGroups(t *testing.T) {
	db := setupModelSyncTestDB(t)
	_ = setupSyncCipher(t)

	provider := models.Provider{Name: "p-nogroups", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err := ListUpstreamModels(context.Background(), repository.New(db), provider.ID)
	if err == nil || !strings.Contains(err.Error(), "no key groups") {
		t.Fatalf("err = %v, want 'no key groups'", err)
	}
}

// TestListUpstreamModels_ProviderNotFound 供应商不存在 → 透传 gorm.ErrRecordNotFound
// （handler 据此区分 404「Provider not found」与上游失败）。
func TestListUpstreamModels_ProviderNotFound(t *testing.T) {
	db := setupModelSyncTestDB(t)
	_ = setupSyncCipher(t)

	_, err := ListUpstreamModels(context.Background(), repository.New(db), 4242)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
	if !strings.Contains(err.Error(), "record not found") {
		t.Fatalf("err = %v, want record not found", err)
	}
}
