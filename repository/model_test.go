package repository

import (
	"context"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.Model{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// UpdateFields 必须能写入零值（GORM 结构体 Updates 会跳过零值）。
func TestModelRepo_UpdateFields_WritesZeroValues(t *testing.T) {
	ctx := context.Background()
	db := newModelTestDB(t)
	repo := NewModelRepo(db)

	m := &models.Model{Name: "gpt-4o", Remark: "some remark", MaxRetry: 3, TimeOut: 30}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 清空 Remark，MaxRetry/TimeOut 设 0
	updates := map[string]any{
		"remark":    "",
		"max_retry": 0,
		"time_out":  0,
	}
	if _, err := repo.UpdateFields(ctx, m.ID, updates); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}

	got, err := repo.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Remark != "" {
		t.Fatalf("Remark = %q, want empty", got.Remark)
	}
	if got.MaxRetry != 0 {
		t.Fatalf("MaxRetry = %d, want 0", got.MaxRetry)
	}
	if got.TimeOut != 0 {
		t.Fatalf("TimeOut = %d, want 0", got.TimeOut)
	}
}
