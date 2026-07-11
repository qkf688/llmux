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
	ioRepo := NewChatIORepo(db)

	log1 := models.ChatLog{Name: "m1", ProviderName: "p1", Status: "success", Style: "openai"}
	log2 := models.ChatLog{Name: "m1", ProviderName: "p1", Status: "error", Style: "openai"}
	if err := db.Create(&log1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&log2).Error; err != nil {
		t.Fatal(err)
	}
	_ = ioRepo // silence if unused after create
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