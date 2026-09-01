package modelsync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/internal/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testSyncCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func setupModelSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:model_sync_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB := testutil.ConfigureSQLiteForSingleConn(t, db)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&models.Provider{},
		&models.ModelSyncLog{},
		&models.Setting{},
		&models.Endpoint{},
		&models.KeyGroup{},
		&models.Credential{},
		&models.Pool{},
	); err != nil {
		t.Fatalf("failed to migrate model sync tables: %v", err)
	}

	return db
}

func setupSyncCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testSyncCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

func mustEncryptKey(t *testing.T, cipher *credentialcrypto.Cipher, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return enc
}

func modelsListHandler(body string, hits *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func seedOpenAIChannel(t *testing.T, db *gorm.DB, cipher *credentialcrypto.Cipher, provider models.Provider, baseURL string, groupCreds []string) (models.Provider, []models.KeyGroup) {
	t.Helper()
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{
		ProviderID: provider.ID,
		Protocol:   "openai",
		URL:        baseURL,
		Enabled:    true,
	}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	groups := make([]models.KeyGroup, 0, len(groupCreds))
	for i, plain := range groupCreds {
		g := models.KeyGroup{
			ProviderID: provider.ID,
			Name:       fmt.Sprintf("g%d", i+1),
			Weight:     1,
		}
		if err := db.Create(&g).Error; err != nil {
			t.Fatalf("create group: %v", err)
		}
		cred := models.Credential{
			GroupID: &g.ID,
			Key:     mustEncryptKey(t, cipher, plain),
			KeyHash: cipher.Hash(plain),
			Status:  models.CredentialStatusActive,
		}
		if err := db.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
		groups = append(groups, g)
	}
	return provider, groups
}

func TestSyncKeyGroup_WritesWhitelistAndHitsOnce(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandler(
		`{"object":"list","data":[{"id":"m1","object":"model"},{"id":"m2","object":"model"}]}`,
		&hits,
	))
	t.Cleanup(server.Close)

	provider, groups := seedOpenAIChannel(t, db, cipher, models.Provider{
		Name:   "p1",
		Type:   "openai",
		Config: `{"api_key":"stale","base_url":"https://should-not-use.example"}`,
	}, server.URL, []string{"sk-g1"})

	snapshot, err := svc.assembler.Assemble(ctx, provider)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	out := svc.syncKeyGroup(ctx, provider, snapshot, groups[0], time.Now())
	if out.Skipped || out.Err != nil {
		t.Fatalf("syncKeyGroup skipped=%v err=%v reason=%s", out.Skipped, out.Err, out.SkipReason)
	}
	if hits.Load() != 1 {
		t.Fatalf("Models() hits = %d, want 1", hits.Load())
	}

	updated, err := svc.repos.KeyGroup.Get(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("Get group: %v", err)
	}
	if updated.Models != "m1,m2" {
		t.Fatalf("Models = %q, want %q", updated.Models, "m1,m2")
	}
}

func TestSyncKeyGroup_NoCredentialSkipsWithoutHit(t *testing.T) {
	db := setupModelSyncTestDB(t)
	_ = setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandler(`{"object":"list","data":[]}`, &hits))
	t.Cleanup(server.Close)

	provider := models.Provider{Name: "p-skip", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: server.URL, Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	g := models.KeyGroup{ProviderID: provider.ID, Name: "empty", Weight: 1}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	snapshot, err := svc.assembler.Assemble(ctx, provider)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	out := svc.syncKeyGroup(ctx, provider, snapshot, g, time.Now())
	if !out.Skipped {
		t.Fatalf("expected skip, got err=%v models=%v", out.Err, out.ModelIDs)
	}
	if hits.Load() != 0 {
		t.Fatalf("Models() hits = %d, want 0", hits.Load())
	}
}

func TestSyncProviderModels_TwoGroupsHitsTwiceAndFillsWhitelist(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandler(
		`{"object":"list","data":[{"id":"a","object":"model"},{"id":"b","object":"model"}]}`,
		&hits,
	))
	t.Cleanup(server.Close)

	provider, groups := seedOpenAIChannel(t, db, cipher, models.Provider{
		Name:   "p2",
		Type:   "openai",
		Config: `{"base_url":"https://unused.example"}`,
	}, server.URL, []string{"sk-1", "sk-2"})

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "success" {
		t.Fatalf("status = %q error=%q, want success", syncLog.Status, syncLog.Error)
	}
	if hits.Load() != 2 {
		t.Fatalf("Models() hits = %d, want 2 (one per group)", hits.Load())
	}
	if syncLog.AddedCount != 2 {
		t.Fatalf("AddedCount = %d, want 2", syncLog.AddedCount)
	}

	for _, g := range groups {
		got, err := svc.repos.KeyGroup.Get(ctx, g.ID)
		if err != nil {
			t.Fatalf("Get group %d: %v", g.ID, err)
		}
		if got.Models != "a,b" {
			t.Fatalf("group %d Models = %q, want a,b", g.ID, got.Models)
		}
	}

	// 不再写 Config.upstream_models
	var updated models.Provider
	if err := db.First(&updated, provider.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if strings.Contains(updated.Config, "upstream_models") {
		t.Fatalf("Config should not gain upstream_models, got %s", updated.Config)
	}
}

func TestSyncProviderModels_SkipsEmptyGroupContinuesOthers(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	var hits atomic.Int64
	server := httptest.NewServer(modelsListHandler(
		`{"object":"list","data":[{"id":"only","object":"model"}]}`,
		&hits,
	))
	t.Cleanup(server.Close)

	provider, groups := seedOpenAIChannel(t, db, cipher, models.Provider{
		Name: "p-partial", Type: "openai", Config: `{}`,
	}, server.URL, []string{"sk-ok"})

	empty := models.KeyGroup{ProviderID: provider.ID, Name: "no-key", Weight: 1}
	if err := db.Create(&empty).Error; err != nil {
		t.Fatalf("create empty group: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "success" {
		t.Fatalf("status = %q error=%q, want success with partial skip", syncLog.Status, syncLog.Error)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
	if !strings.Contains(syncLog.Error, "no-key") {
		t.Fatalf("Error should mention skipped group, got %q", syncLog.Error)
	}

	got, err := svc.repos.KeyGroup.Get(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Models != "only" {
		t.Fatalf("Models = %q, want only", got.Models)
	}
}

func TestSyncProviderModels_AllGroupsSkippedIsError(t *testing.T) {
	db := setupModelSyncTestDB(t)
	_ = setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	provider := models.Provider{Name: "p-none", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: "http://127.0.0.1:9", Enabled: true}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	g := models.KeyGroup{ProviderID: provider.ID, Name: "empty", Weight: 1}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "error" {
		t.Fatalf("status = %q, want error", syncLog.Status)
	}
}

func TestSyncProviderModels_NoGroupsIsError(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	provider := models.Provider{Name: "p-nogroup", Type: "openai", Config: `{}`}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "error" {
		t.Fatalf("status = %q, want error", syncLog.Status)
	}
}

func TestSyncProviderModels_FullReplaceWhitelist(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	server := httptest.NewServer(modelsListHandler(
		`{"object":"list","data":[{"id":"new1","object":"model"}]}`,
		nil,
	))
	t.Cleanup(server.Close)

	provider, groups := seedOpenAIChannel(t, db, cipher, models.Provider{
		Name: "p-replace", Type: "openai", Config: `{}`,
	}, server.URL, []string{"sk"})
	if _, err := svc.repos.KeyGroup.UpdateFields(ctx, groups[0].ID, map[string]any{"models": "old1,old2"}); err != nil {
		t.Fatalf("seed old models: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "success" {
		t.Fatalf("status = %q error=%q", syncLog.Status, syncLog.Error)
	}

	got, err := svc.repos.KeyGroup.Get(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Models != "new1" {
		t.Fatalf("Models = %q, want full replace new1", got.Models)
	}
}

func TestSyncProviderModels_FilterRulesFailureDoesNotWriteWhitelist(t *testing.T) {
	db := setupModelSyncTestDB(t)
	cipher := setupSyncCipher(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	server := httptest.NewServer(modelsListHandler(
		`{"object":"list","data":[{"id":"m1","object":"model"},{"id":"m2","object":"model"}]}`,
		nil,
	))
	t.Cleanup(server.Close)

	filterOn := true
	provider, groups := seedOpenAIChannel(t, db, cipher, models.Provider{
		Name: "p-filter-fail", Type: "openai", Config: `{}`, ModelFilterEnabled: &filterOn,
	}, server.URL, []string{"sk"})
	if _, err := svc.repos.KeyGroup.UpdateFields(ctx, groups[0].ID, map[string]any{"models": "seed-only"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	// 未写入 SettingKeyModelSyncFilterRules → filterModelsByRules 应失败，不得落库未过滤列表

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}
	if syncLog.Status != "error" {
		t.Fatalf("status = %q error=%q, want error when filter rules unreadable", syncLog.Status, syncLog.Error)
	}

	got, err := svc.repos.KeyGroup.Get(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Models != "seed-only" {
		t.Fatalf("Models = %q, want unchanged seed-only (no silent unfiltered write)", got.Models)
	}
}

func TestSyncAllProviders_SkipsDisabledModelEndpoint(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	disabled := false
	enabled := true
	providers := []models.Provider{
		{Name: "p-disabled", Type: "unsupported", Config: `{"api_key":"k1"}`, ModelEndpoint: &disabled},
		{Name: "p-enabled", Type: "unsupported", Config: `{"api_key":"k2"}`, ModelEndpoint: &enabled},
	}
	for i := range providers {
		if err := db.Create(&providers[i]).Error; err != nil {
			t.Fatalf("failed to create provider %d: %v", i, err)
		}
	}

	logs, err := svc.SyncAllProviders(ctx)
	if err != nil {
		t.Fatalf("SyncAllProviders() error = %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs length = %d, want 1", len(logs))
	}
	if logs[0].ProviderName != "p-enabled" {
		t.Fatalf("synced provider = %q, want %q", logs[0].ProviderName, "p-enabled")
	}
}

func TestSyncAllProviders_AssignsSameBatchID(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	providers := []models.Provider{
		{Name: "p1", Type: "unsupported", Config: `{"api_key":"k1"}`},
		{Name: "p2", Type: "unsupported", Config: `{"api_key":"k2"}`},
	}
	for i := range providers {
		if err := db.Create(&providers[i]).Error; err != nil {
			t.Fatalf("failed to create provider %d: %v", i, err)
		}
	}

	logs, err := svc.SyncAllProviders(ctx)
	if err != nil {
		t.Fatalf("SyncAllProviders() error = %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs length = %d, want 2", len(logs))
	}

	if logs[0].BatchID == nil || logs[1].BatchID == nil {
		t.Fatal("expected non-nil BatchID")
	}
	if *logs[0].BatchID != *logs[1].BatchID {
		t.Fatalf("batch IDs differ: %q vs %q", *logs[0].BatchID, *logs[1].BatchID)
	}
}

func TestSelectSyncEndpoint_PrefersMatchingEnabledMinID(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: "anthropic", Enabled: true},
		{Model: gorm.Model{ID: 2}, Protocol: "openai", Enabled: false},
		{Model: gorm.Model{ID: 3}, Protocol: "openai", Enabled: true},
		{Model: gorm.Model{ID: 4}, Protocol: "openai", Enabled: true},
	}
	got, ok := selectSyncEndpoint("openai", eps)
	if !ok {
		t.Fatal("expected endpoint")
	}
	if got.ID != 3 {
		t.Fatalf("ID = %d, want 3 (first enabled openai)", got.ID)
	}
}

func TestMergeProviderModelCatalog(t *testing.T) {
	groups := []models.KeyGroup{
		{Models: "a, b"},
		{Models: ""}, // empty = unlimited, contributes nothing
		{Models: "b,c"},
	}
	got := mergeProviderModelCatalog(`{"custom_models":["c","d"],"upstream_models":["ignore"]}`, groups)
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGetRecentAddedModels_NoBatchLogs(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})

	result, err := svc.GetRecentAddedModels(context.Background())
	if err != nil {
		t.Fatalf("GetRecentAddedModels() error = %v", err)
	}
	if result.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", result.TotalCount)
	}
	if len(result.Data) != 0 {
		t.Fatalf("Data length = %d, want 0", len(result.Data))
	}
	if result.SyncTime != nil {
		t.Fatal("SyncTime should be nil when no batch logs")
	}
}

func TestGetRecentAddedModels_LatestBatchOnly(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})

	oldBatch := "100"
	newBatch := "200"
	now := time.Now()

	logs := []models.ModelSyncLog{
		{
			BatchID:      &oldBatch,
			ProviderID:   1,
			ProviderName: "old",
			Status:       "success",
			AddedCount:   1,
			AddedModels:  []string{"old-model"},
			SyncedAt:     now.Add(-2 * time.Hour),
		},
		{
			BatchID:      &newBatch,
			ProviderID:   2,
			ProviderName: "new-a",
			Status:       "success",
			AddedCount:   1,
			AddedModels:  []string{"new-model-a"},
			SyncedAt:     now.Add(-30 * time.Minute),
		},
		{
			BatchID:      &newBatch,
			ProviderID:   3,
			ProviderName: "new-b",
			Status:       "success",
			AddedCount:   1,
			AddedModels:  []string{"new-model-b"},
			SyncedAt:     now.Add(-10 * time.Minute),
		},
		{
			BatchID:      &newBatch,
			ProviderID:   4,
			ProviderName: "new-c",
			Status:       "unchanged",
			AddedCount:   0,
			AddedModels:  []string{},
			SyncedAt:     now.Add(-5 * time.Minute),
		},
	}

	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("failed to create log %d: %v", i, err)
		}
	}

	result, err := svc.GetRecentAddedModels(context.Background())
	if err != nil {
		t.Fatalf("GetRecentAddedModels() error = %v", err)
	}

	if result.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", result.TotalCount)
	}
	if len(result.Data) != 2 {
		t.Fatalf("Data length = %d, want 2", len(result.Data))
	}
	if result.Data[0].AddedAt.Before(result.Data[1].AddedAt) {
		t.Fatalf("expected descending sort by AddedAt, got %v then %v", result.Data[0].AddedAt, result.Data[1].AddedAt)
	}
	if result.SyncTime == nil {
		t.Fatal("SyncTime should not be nil")
	}
}
