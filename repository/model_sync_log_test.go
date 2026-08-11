package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newSyncLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelSyncLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedSyncLogs 直接建行；SyncedAt 显式给值以保证排序可预期。
func seedSyncLogs(t *testing.T, db *gorm.DB, logs []models.ModelSyncLog) {
	t.Helper()
	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("seed log: %v", err)
		}
	}
}

func TestModelSyncLogRepo_AggregateProviderStats_EmptyProviderIDs(t *testing.T) {
	// providerIDs 为空必须返回零值统计，不能退化成全表聚合。
	ctx := context.Background()
	db := newSyncLogTestDB(t)
	seedSyncLogs(t, db, []models.ModelSyncLog{
		{ProviderID: 1, Status: "success", AddedCount: 2, SyncedAt: time.Now()},
	})

	stats, err := NewModelSyncLogRepo(db).AggregateProviderStats(ctx, nil)
	if err != nil {
		t.Fatalf("AggregateProviderStats error: %v", err)
	}
	if stats.ProvidersSynced != 0 || stats.ProvidersWithUpdates != 0 {
		t.Errorf("空 providerIDs 应返回零值统计，got %+v", stats)
	}
	if stats.LastSyncAt != nil {
		t.Errorf("LastSyncAt = %v, want nil", stats.LastSyncAt)
	}
}

func TestModelSyncLogRepo_AggregateProviderStats_LatestPerProvider(t *testing.T) {
	// 同一 provider 的多条历史只算最新那条：p1 最新是 error（更晚），旧的 success 不该被计。
	ctx := context.Background()
	db := newSyncLogTestDB(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	seedSyncLogs(t, db, []models.ModelSyncLog{
		{ProviderID: 1, Status: "success", AddedCount: 3, SyncedAt: base},
		{ProviderID: 1, Status: "error", Error: "boom", SyncedAt: base.Add(time.Hour)},
		{ProviderID: 2, Status: "unchanged", SyncedAt: base.Add(2 * time.Hour)},
		{ProviderID: 3, Status: "success", AddedCount: 1, SyncedAt: base.Add(30 * time.Minute)},
	})

	stats, err := NewModelSyncLogRepo(db).AggregateProviderStats(ctx, []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("AggregateProviderStats error: %v", err)
	}
	if stats.ProvidersWithErrors != 1 {
		t.Errorf("ProvidersWithErrors = %d, want 1 (p1 最新为 error)", stats.ProvidersWithErrors)
	}
	if stats.ProvidersUnchanged != 1 {
		t.Errorf("ProvidersUnchanged = %d, want 1 (p2)", stats.ProvidersUnchanged)
	}
	if stats.ProvidersWithUpdates != 1 {
		t.Errorf("ProvidersWithUpdates = %d, want 1 (p3；p1 的旧 success 不计)", stats.ProvidersWithUpdates)
	}
	if stats.ProvidersSynced != 3 {
		t.Errorf("ProvidersSynced = %d, want 3", stats.ProvidersSynced)
	}
	if stats.LastSyncAt == nil || !stats.LastSyncAt.Equal(base.Add(2*time.Hour)) {
		t.Errorf("LastSyncAt = %v, want %v", stats.LastSyncAt, base.Add(2*time.Hour))
	}
}

func TestModelSyncLogRepo_AggregateProviderStats_LegacyRowsWithoutStatus(t *testing.T) {
	// 向后兼容：早期行没有 Status 字段（空串），按增删数量反推。
	// 这条兼容规则若丢失，老库统计会整体漂移到 unchanged 或全丢。
	ctx := context.Background()
	db := newSyncLogTestDB(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	seedSyncLogs(t, db, []models.ModelSyncLog{
		{ProviderID: 1, Status: "", AddedCount: 2, SyncedAt: base},                    // → success
		{ProviderID: 2, Status: "", RemovedCount: 1, SyncedAt: base.Add(time.Minute)}, // → success
		{ProviderID: 3, Status: "", SyncedAt: base.Add(2 * time.Minute)},              // → unchanged
	})

	stats, err := NewModelSyncLogRepo(db).AggregateProviderStats(ctx, []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("AggregateProviderStats error: %v", err)
	}
	if stats.ProvidersWithUpdates != 2 {
		t.Errorf("ProvidersWithUpdates = %d, want 2（空 status + 有增删 → success）", stats.ProvidersWithUpdates)
	}
	if stats.ProvidersUnchanged != 1 {
		t.Errorf("ProvidersUnchanged = %d, want 1（空 status + 无增删 → unchanged）", stats.ProvidersUnchanged)
	}
	if stats.ProvidersWithErrors != 0 {
		t.Errorf("ProvidersWithErrors = %d, want 0", stats.ProvidersWithErrors)
	}
}

func TestModelSyncLogRepo_AggregateProviderStats_ExcludesUnlistedProviders(t *testing.T) {
	// 未在 providerIDs 中的 provider（例如已关闭模型端点）不得进入统计。
	ctx := context.Background()
	db := newSyncLogTestDB(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	seedSyncLogs(t, db, []models.ModelSyncLog{
		{ProviderID: 1, Status: "success", AddedCount: 1, SyncedAt: base},
		{ProviderID: 99, Status: "error", SyncedAt: base.Add(time.Hour)},
	})

	stats, err := NewModelSyncLogRepo(db).AggregateProviderStats(ctx, []uint{1})
	if err != nil {
		t.Fatalf("AggregateProviderStats error: %v", err)
	}
	if stats.ProvidersSynced != 1 || stats.ProvidersWithErrors != 0 {
		t.Errorf("未列入的 p99 泄漏进统计，got %+v", stats)
	}
	if stats.LastSyncAt == nil || !stats.LastSyncAt.Equal(base) {
		t.Errorf("LastSyncAt = %v, want %v（不应取 p99 的更晚时间）", stats.LastSyncAt, base)
	}
}

func TestModelSyncLogRepo_AggregateProviderStats_ZeroSyncedAt(t *testing.T) {
	// 全部 SyncedAt 为零值时：provider 仍计入 seen/状态桶，但 LastSyncAt 保持 nil。
	// 该分支对应「provider 有同步记录但时间字段为历史零值行」的边界场景。
	ctx := context.Background()
	db := newSyncLogTestDB(t)
	seedSyncLogs(t, db, []models.ModelSyncLog{
		{ProviderID: 1, Status: "success", AddedCount: 2}, // SyncedAt 零值
		{ProviderID: 2, Status: "unchanged"},              // SyncedAt 零值
	})

	stats, err := NewModelSyncLogRepo(db).AggregateProviderStats(ctx, []uint{1, 2})
	if err != nil {
		t.Fatalf("AggregateProviderStats error: %v", err)
	}
	if stats.ProvidersSynced != 2 {
		t.Errorf("ProvidersSynced = %d, want 2（零值 SyncedAt 仍计入 seen）", stats.ProvidersSynced)
	}
	if stats.ProvidersWithUpdates != 1 {
		t.Errorf("ProvidersWithUpdates = %d, want 1", stats.ProvidersWithUpdates)
	}
	if stats.ProvidersUnchanged != 1 {
		t.Errorf("ProvidersUnchanged = %d, want 1", stats.ProvidersUnchanged)
	}
	if stats.LastSyncAt != nil {
		t.Errorf("LastSyncAt = %v, want nil（全部 SyncedAt 零值时不应有值）", stats.LastSyncAt)
	}
}

func TestNormalizeSyncStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		addedCount   int
		removedCount int
		want         string
	}{
		{"显式 success 原样返回", "success", 0, 0, "success"},
		{"显式 error 原样返回", "error", 0, 0, "error"},
		{"显式 unchanged 原样返回", "unchanged", 5, 5, "unchanged"},
		{"旧行有新增 → success", "", 3, 0, "success"},
		{"旧行有删除 → success", "", 0, 2, "success"},
		{"旧行无增删 → unchanged", "", 0, 0, "unchanged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSyncStatus(tt.status, tt.addedCount, tt.removedCount); got != tt.want {
				t.Errorf("normalizeSyncStatus(%q, %d, %d) = %q, want %q",
					tt.status, tt.addedCount, tt.removedCount, got, tt.want)
			}
		})
	}
}

func TestProviderRepo_EnabledModelEndpoint_NullRowsCountAsEnabled(t *testing.T) {
	// NULL 语义为「默认启用」；若新方法漏了 NULL 分支，Stats 的总数会凭空少掉历史 provider。
	ctx := context.Background()
	repo := NewProviderRepo(newTestDB(t))

	trueVal := true
	falseVal := false
	for _, p := range []*models.Provider{
		{Name: "p-true", Type: "openai", Config: `{}`, ModelEndpoint: &trueVal},
		{Name: "p-null", Type: "openai", Config: `{}`}, // NULL → 视为启用
		{Name: "p-false", Type: "openai", Config: `{}`, ModelEndpoint: &falseVal},
	} {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	count, err := repo.CountEnabledModelEndpoint(ctx)
	if err != nil {
		t.Fatalf("CountEnabledModelEndpoint error: %v", err)
	}
	if count != 2 {
		t.Errorf("CountEnabledModelEndpoint = %d, want 2 (p-true + p-null)", count)
	}

	ids, err := repo.EnabledModelEndpointIDs(ctx)
	if err != nil {
		t.Fatalf("EnabledModelEndpointIDs error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("EnabledModelEndpointIDs 返回 %d 个 ID，want 2", len(ids))
	}
}
