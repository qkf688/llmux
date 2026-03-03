package modelsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupModelSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:model_sync_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB := configureModelSyncSQLite(t, db)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&models.Provider{},
		&models.ModelSyncLog{},
		&models.Setting{},
	); err != nil {
		t.Fatalf("failed to migrate model sync tables: %v", err)
	}

	return db
}

func configureModelSyncSQLite(t *testing.T, db *gorm.DB) *sql.DB {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return sqlDB
}

func TestSyncProviderModels_ModelEndpointDisabledStillSyncs(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"m1","object":"model","created":0,"owned_by":"o"},{"id":"m2","object":"model","created":0,"owned_by":"o"}]}`))
	}))
	t.Cleanup(server.Close)

	disabled := false
	provider := models.Provider{
		Name:          "test-provider",
		Type:          "openai",
		Config:        fmt.Sprintf(`{"api_key":"k","base_url":"%s"}`, server.URL),
		ModelEndpoint: &disabled,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels() error = %v", err)
	}
	if syncLog == nil {
		t.Fatal("SyncProviderModels() returned nil log")
	}
	if syncLog.Status != "success" {
		t.Fatalf("log status = %q, want %q (error=%q)", syncLog.Status, "success", syncLog.Error)
	}
	if syncLog.AddedCount != 2 {
		t.Fatalf("added count = %d, want 2", syncLog.AddedCount)
	}
	if !reflect.DeepEqual(syncLog.AddedModels, []string{"m1", "m2"}) {
		t.Fatalf("added models = %v, want %v", syncLog.AddedModels, []string{"m1", "m2"})
	}

	var count int64
	if err := db.Model(&models.ModelSyncLog{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count sync logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("sync log count = %d, want 1", count)
	}

	var updated models.Provider
	if err := db.First(&updated, provider.ID).Error; err != nil {
		t.Fatalf("failed to fetch updated provider: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(updated.Config), &parsed); err != nil {
		t.Fatalf("failed to parse updated provider config: %v", err)
	}
	upstreamAny, ok := parsed["upstream_models"].([]any)
	if !ok {
		t.Fatalf("upstream_models missing or invalid type: %T", parsed["upstream_models"])
	}
	got := make([]string, 0, len(upstreamAny))
	for _, item := range upstreamAny {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("unexpected upstream_models item type: %T", item)
		}
		got = append(got, s)
	}
	if !reflect.DeepEqual(got, []string{"m1", "m2"}) {
		t.Fatalf("provider upstream_models = %v, want %v", got, []string{"m1", "m2"})
	}
}

func TestSyncProviderModels_UnsupportedProviderType(t *testing.T) {
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})
	ctx := context.Background()

	provider := models.Provider{
		Name:   "unsupported-provider",
		Type:   "unsupported",
		Config: `{"api_key":"k"}`,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	syncLog, err := svc.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("SyncProviderModels() error = %v", err)
	}
	if syncLog == nil {
		t.Fatal("SyncProviderModels() returned nil log")
	}
	if syncLog.Status != "error" {
		t.Fatalf("log status = %q, want %q", syncLog.Status, "error")
	}
	if syncLog.Error == "" {
		t.Fatal("expected non-empty error message for unsupported provider type")
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
		t.Fatal("expected non-nil batch_id on all logs")
	}
	if *logs[0].BatchID != *logs[1].BatchID {
		t.Fatalf("batch_id mismatch: %q vs %q", *logs[0].BatchID, *logs[1].BatchID)
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
