package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(
		&models.ChatLog{},
		&models.ChatIO{},
		&models.HealthCheckLog{},
		&models.ModelSyncLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestChatLogRepo_ListAndHardDeleteFiltered(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	chatRepo := NewChatLogRepo(db)

	log1 := models.ChatLog{Name: "m1", ProviderName: "p1", Status: "success", Style: "openai"}
	log2 := models.ChatLog{Name: "m1", ProviderName: "p1", Status: "error", Style: "openai"}
	if err := db.Create(&log1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&log2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatIO{LogId: log1.ID, Input: "in"}).Error; err != nil {
		t.Fatal(err)
	}

	list, err := chatRepo.List(ctx, ChatLogListOptions{
		Filter: ChatLogFilter{Status: "success"},
		Page:   1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("total = %d, want 1", list.Total)
	}

	deleted, err := chatRepo.HardDeleteFiltered(ctx, ChatLogFilter{Status: "success"})
	if err != nil {
		t.Fatalf("HardDeleteFiltered: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var ioCount int64
	db.Unscoped().Model(&models.ChatIO{}).Count(&ioCount)
	if ioCount != 0 {
		t.Fatalf("chat io remaining = %d", ioCount)
	}
}

// TestChatLogRepo_HardDeleteFilteredRejectsEmptyFilter 回归：空筛选会让子查询退化成
// 「全表 id」，把条件清理变成清空 chat_logs + chat_ios。守卫此前只在 handler 里，
// 仓储作为可复用 API 是裸的。
func TestChatLogRepo_HardDeleteFilteredRejectsEmptyFilter(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	keep := models.ChatLog{Name: "m1", ProviderName: "p1", Status: "success", Style: "openai"}
	if err := db.Create(&keep).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatIO{LogId: keep.ID, Input: "in"}).Error; err != nil {
		t.Fatal(err)
	}

	for _, filter := range []ChatLogFilter{
		{},
		{Status: "   "}, // 空白字符不构成条件
	} {
		deleted, err := repo.HardDeleteFiltered(ctx, filter)
		if !errors.Is(err, ErrEmptyChatLogFilter) {
			t.Fatalf("filter %#v: err = %v, want ErrEmptyChatLogFilter", filter, err)
		}
		if deleted != 0 {
			t.Fatalf("filter %#v: deleted = %d, want 0", filter, deleted)
		}
	}

	var logCount, ioCount int64
	db.Unscoped().Model(&models.ChatLog{}).Count(&logCount)
	db.Unscoped().Model(&models.ChatIO{}).Count(&ioCount)
	if logCount != 1 || ioCount != 1 {
		t.Fatalf("rows after rejected deletes: logs=%d ios=%d, want 1/1", logCount, ioCount)
	}
}

func TestChatIORepo_GetByLogID(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatIORepo(db)

	_ = db.Create(&models.ChatIO{LogId: 42, Input: "x"}).Error
	got, err := repo.GetByLogID(ctx, 42)
	if err != nil {
		t.Fatalf("GetByLogID: %v", err)
	}
	if got.Input != "x" {
		t.Fatalf("input = %q", got.Input)
	}
	_, err = repo.GetByLogID(ctx, 99)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestHealthCheckLogRepo_ListAndClear(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewHealthCheckLogRepo(db)

	_ = db.Create(&models.HealthCheckLog{
		ModelProviderID: 1, ModelName: "m", ProviderName: "p", Status: "success", CheckedAt: time.Now(),
	}).Error

	list, err := repo.List(ctx, HealthCheckLogListOptions{Page: 1, PageSize: 10})
	if err != nil || list.Total != 1 {
		t.Fatalf("List: total=%v err=%v", list, err)
	}
	n, err := repo.HardDeleteAll(ctx)
	if err != nil || n != 1 {
		t.Fatalf("HardDeleteAll: n=%d err=%v", n, err)
	}
}

// TestChatLogRepo_EnforceRetentionDeletesChatIO 端到端验证保留策略事务：
// 删 ChatLog 时对应 ChatIO 同步删除，不产生孤儿（待办 12 回归保护）。
func TestChatLogRepo_EnforceRetentionDeletesChatIO(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	chatRepo := NewChatLogRepo(db)

	// 建 4 条 ChatLog，每条配 1 条 ChatIO
	for i := 0; i < 4; i++ {
		log := models.ChatLog{Name: "m", ProviderName: "p", Status: "success", Style: "openai"}
		if err := db.Create(&log).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.ChatIO{LogId: log.ID, Input: "in"}).Error; err != nil {
			t.Fatal(err)
		}
	}

	// retention=2 → 删最旧 2 条（id 1,2），保留 id 3,4
	deleted, err := chatRepo.EnforceRetention(ctx, 2)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	var logCount, ioCount int64
	db.Unscoped().Model(&models.ChatLog{}).Count(&logCount)
	db.Unscoped().Model(&models.ChatIO{}).Count(&ioCount)
	if logCount != 2 {
		t.Fatalf("chat_log remaining = %d, want 2", logCount)
	}
	if ioCount != 2 {
		t.Fatalf("chat_io remaining = %d, want 2 (no orphans)", ioCount)
	}

	// 残留 ChatIO 的 log_id 必须指向存活的 ChatLog，不能是已删的
	var orphanCount int64
	db.Unscoped().Model(&models.ChatIO{}).
		Where("log_id NOT IN (?)", db.Unscoped().Model(&models.ChatLog{}).Select("id")).
		Count(&orphanCount)
	if orphanCount != 0 {
		t.Fatalf("orphan chat_io = %d, want 0", orphanCount)
	}
}

func TestModelSyncLogRepo_ListAndClearErrors(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewModelSyncLogRepo(db)

	now := time.Now()
	_ = db.Create(&models.ModelSyncLog{ProviderID: 1, Status: "success", SyncedAt: now}).Error
	_ = db.Create(&models.ModelSyncLog{ProviderID: 1, Status: "error", SyncedAt: now}).Error
	_ = db.Create(&models.ModelSyncLog{ProviderID: 2, Status: "error", SyncedAt: now}).Error

	list, err := repo.List(ctx, ModelSyncLogListOptions{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("default total = %d, want 1 (success only)", list.Total)
	}

	n, err := repo.HardDeleteErrors(ctx, []uint{1})
	if err != nil || n != 1 {
		t.Fatalf("HardDeleteErrors: n=%d err=%v", n, err)
	}
}
