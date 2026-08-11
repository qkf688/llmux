package modelsync

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// seedStatsFixtures 建 3 个启用模型端点的 provider，其中 p1/p2 有同步日志、p3 从未同步。
// 返回值为「预期的 last sync 时间」，供 nextSyncAt 断言复用。
func seedStatsFixtures(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()

	enabled := true
	providers := []models.Provider{
		{Name: "p1", Type: "openai", Config: `{}`, ModelEndpoint: &enabled},
		{Name: "p2", Type: "openai", Config: `{}`, ModelEndpoint: &enabled},
		{Name: "p3-never-synced", Type: "openai", Config: `{}`, ModelEndpoint: &enabled},
	}
	for i := range providers {
		if err := db.Create(&providers[i]).Error; err != nil {
			t.Fatalf("create provider %d: %v", i, err)
		}
	}

	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	latest := base.Add(time.Hour)
	logs := []models.ModelSyncLog{
		{ProviderID: providers[0].ID, Status: "success", AddedCount: 2, SyncedAt: base},
		{ProviderID: providers[1].ID, Status: "error", Error: "boom", SyncedAt: latest},
	}
	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("create log %d: %v", i, err)
		}
	}

	return latest
}

func setSetting(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	if err := db.Create(&models.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatalf("create setting %s: %v", key, err)
	}
}

func TestGetSyncStats_NeverSyncedAndBuckets(t *testing.T) {
	// 端到端口径：3 个启用 provider，2 个有日志（1 success + 1 error），1 个从未同步。
	// neverSynced = total - synced 是 handler 重构后唯一的派生计数，必须锁死。
	db := setupModelSyncTestDB(t)
	latest := seedStatsFixtures(t, db)
	svc := NewService(db, ActionHooks{})

	stats, err := svc.GetSyncStats(context.Background(), repository.New(db))
	if err != nil {
		t.Fatalf("GetSyncStats() error = %v", err)
	}

	if stats.TotalProviders != 3 {
		t.Errorf("TotalProviders = %d, want 3", stats.TotalProviders)
	}
	if stats.ProvidersWithUpdates != 1 {
		t.Errorf("ProvidersWithUpdates = %d, want 1", stats.ProvidersWithUpdates)
	}
	if stats.ProvidersWithErrors != 1 {
		t.Errorf("ProvidersWithErrors = %d, want 1", stats.ProvidersWithErrors)
	}
	if stats.ProvidersUnchanged != 0 {
		t.Errorf("ProvidersUnchanged = %d, want 0", stats.ProvidersUnchanged)
	}
	if stats.ProvidersNeverSynced != 1 {
		t.Errorf("ProvidersNeverSynced = %d, want 1 (p3)", stats.ProvidersNeverSynced)
	}
	if stats.LastSyncAt == nil || !stats.LastSyncAt.Equal(latest) {
		t.Errorf("LastSyncAt = %v, want %v", stats.LastSyncAt, latest)
	}
}

func TestGetSyncStats_NextSyncAtOnlyWhenEnabled(t *testing.T) {
	tests := []struct {
		name         string
		syncEnabled  string
		syncInterval string
		wantEnabled  bool
		wantInterval int
		wantNextSync bool
	}{
		{
			name:         "关闭自动同步则 NextSyncAt 为 nil",
			syncEnabled:  "false",
			syncInterval: "6",
			wantEnabled:  false,
			wantInterval: 6,
			wantNextSync: false,
		},
		{
			name:         "开启自动同步则 NextSyncAt = LastSyncAt + interval",
			syncEnabled:  "true",
			syncInterval: "6",
			wantEnabled:  true,
			wantInterval: 6,
			wantNextSync: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupModelSyncTestDB(t)
			latest := seedStatsFixtures(t, db)
			setSetting(t, db, models.SettingKeyModelSyncEnabled, tt.syncEnabled)
			setSetting(t, db, models.SettingKeyModelSyncInterval, tt.syncInterval)
			svc := NewService(db, ActionHooks{})

			stats, err := svc.GetSyncStats(context.Background(), repository.New(db))
			if err != nil {
				t.Fatalf("GetSyncStats() error = %v", err)
			}

			if stats.SyncEnabled != tt.wantEnabled {
				t.Errorf("SyncEnabled = %v, want %v", stats.SyncEnabled, tt.wantEnabled)
			}
			if stats.SyncInterval != tt.wantInterval {
				t.Errorf("SyncInterval = %d, want %d", stats.SyncInterval, tt.wantInterval)
			}
			if tt.wantNextSync {
				want := latest.Add(time.Duration(tt.wantInterval) * time.Hour)
				if stats.NextSyncAt == nil || !stats.NextSyncAt.Equal(want) {
					t.Errorf("NextSyncAt = %v, want %v", stats.NextSyncAt, want)
				}
			} else if stats.NextSyncAt != nil {
				t.Errorf("NextSyncAt = %v, want nil（未启用自动同步）", stats.NextSyncAt)
			}
		})
	}
}

func TestGetSyncStats_MissingIntervalFallsBackTo12(t *testing.T) {
	// 设置表无 interval 记录时应回落到 12 小时默认值，与重构前 handler 一致。
	db := setupModelSyncTestDB(t)
	latest := seedStatsFixtures(t, db)
	setSetting(t, db, models.SettingKeyModelSyncEnabled, "true")
	svc := NewService(db, ActionHooks{})

	stats, err := svc.GetSyncStats(context.Background(), repository.New(db))
	if err != nil {
		t.Fatalf("GetSyncStats() error = %v", err)
	}

	if stats.SyncInterval != 12 {
		t.Errorf("SyncInterval = %d, want 12（默认值）", stats.SyncInterval)
	}
	want := latest.Add(12 * time.Hour)
	if stats.NextSyncAt == nil || !stats.NextSyncAt.Equal(want) {
		t.Errorf("NextSyncAt = %v, want %v", stats.NextSyncAt, want)
	}
}

func TestGetSyncStats_NoProviders(t *testing.T) {
	// 空库：所有计数为 0，LastSyncAt / NextSyncAt 均为 nil，且不因空 providerIDs 报错。
	db := setupModelSyncTestDB(t)
	svc := NewService(db, ActionHooks{})

	stats, err := svc.GetSyncStats(context.Background(), repository.New(db))
	if err != nil {
		t.Fatalf("GetSyncStats() error = %v", err)
	}

	if stats.TotalProviders != 0 || stats.ProvidersNeverSynced != 0 {
		t.Errorf("空库统计应全零，got %+v", stats)
	}
	if stats.LastSyncAt != nil || stats.NextSyncAt != nil {
		t.Errorf("LastSyncAt=%v NextSyncAt=%v，want 均为 nil", stats.LastSyncAt, stats.NextSyncAt)
	}
}
