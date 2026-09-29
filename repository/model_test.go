package repository

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.New(t, &models.Model{})
}

// UpdateFields 必须能写入零值（GORM 结构体 Updates 会跳过零值）。
func TestModelRepo_UpdateFields_WritesZeroValues(t *testing.T) {
	ctx := context.Background()
	db := newModelTestDB(t)
	repo := NewModelRepo(db)

	m := &models.Model{Name: "gpt-4o", Remark: "some remark"}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 清空 Remark（零值写回）
	updates := map[string]any{
		"remark": "",
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
}
