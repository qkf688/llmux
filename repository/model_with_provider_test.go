package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAssocTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelWithProvider{}, &models.ModelTemplateItem{}, &models.Provider{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestModelWithProviderRepo_CRUDAndBatch(t *testing.T) {
	ctx := context.Background()
	repo := NewModelWithProviderRepo(newAssocTestDB(t))

	trueVal := true
	assoc := &models.ModelWithProvider{
		ModelID:       1,
		ProviderID:    10,
		ProviderModel: "gpt-4",
		Status:        &trueVal,
		Weight:        5,
		Priority:      10,
	}
	if err := repo.Create(ctx, assoc); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if assoc.ID == 0 {
		t.Fatal("Create should set ID")
	}

	got, err := repo.Get(ctx, assoc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ProviderModel != "gpt-4" {
		t.Errorf("ProviderModel = %q, want gpt-4", got.ProviderModel)
	}

	list, err := repo.ListByModelID(ctx, 1)
	if err != nil {
		t.Fatalf("ListByModelID: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByModelID len = %d, want 1", len(list))
	}

	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAll len = %d, want 1", len(all))
	}

	if err := repo.Update(ctx, assoc.ID, models.ModelWithProvider{ProviderModel: "gpt-4o", Weight: 8}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, assoc.ID)
	if got.ProviderModel != "gpt-4o" || got.Weight != 8 {
		t.Errorf("after Update: model=%q weight=%d", got.ProviderModel, got.Weight)
	}

	falseVal := false
	affected, err := repo.UpdateByIDs(ctx, []uint{assoc.ID}, models.ModelWithProvider{Status: &falseVal})
	if err != nil {
		t.Fatalf("UpdateByIDs: %v", err)
	}
	if affected != 1 {
		t.Errorf("UpdateByIDs affected = %d, want 1", affected)
	}

	assoc2 := &models.ModelWithProvider{ModelID: 2, ProviderID: 10, ProviderModel: "claude", Status: &trueVal}
	if err := repo.Create(ctx, assoc2); err != nil {
		t.Fatalf("Create assoc2: %v", err)
	}

	deleted, err := repo.DeleteByIDs(ctx, []uint{assoc.ID})
	if err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}
	if deleted != 1 {
		t.Errorf("DeleteByIDs = %d, want 1", deleted)
	}

	deleted, err = repo.DeleteByProviderID(ctx, 10)
	if err != nil {
		t.Fatalf("DeleteByProviderID: %v", err)
	}
	if deleted != 1 {
		t.Errorf("DeleteByProviderID = %d, want 1", deleted)
	}

	_, err = repo.Get(ctx, assoc2.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Get after DeleteByProviderID: err=%v, want ErrRecordNotFound", err)
	}
}

func TestModelTemplateItemRepo_CountCreateListHardDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewModelTemplateItemRepo(newAssocTestDB(t))

	count, err := repo.CountByModelIDAndName(ctx, 1, "gpt-4")
	if err != nil {
		t.Fatalf("Count empty: %v", err)
	}
	if count != 0 {
		t.Errorf("Count empty = %d, want 0", count)
	}

	item := &models.ModelTemplateItem{ModelID: 1, Name: "gpt-4"}
	if err := repo.Create(ctx, item); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err = repo.CountByModelIDAndName(ctx, 1, "gpt-4")
	if err != nil {
		t.Fatalf("Count after create: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	items, err := repo.ListByModelID(ctx, 1)
	if err != nil {
		t.Fatalf("ListByModelID: %v", err)
	}
	if len(items) != 1 || items[0].Name != "gpt-4" {
		t.Fatalf("ListByModelID = %+v", items)
	}

	// soft-delete 会挡 unique；硬删后应可重建
	deleted, err := repo.DeleteByModelIDAndNameUnscoped(ctx, 1, "gpt-4")
	if err != nil {
		t.Fatalf("DeleteByModelIDAndNameUnscoped: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	if err := repo.Create(ctx, &models.ModelTemplateItem{ModelID: 1, Name: "gpt-4"}); err != nil {
		t.Fatalf("recreate after hard delete: %v", err)
	}
}

func TestModelTemplateItemRepo_DeleteByModelIDUnscopedAllowsRecreate(t *testing.T) {
	ctx := context.Background()
	db := newAssocTestDB(t)
	repo := NewModelTemplateItemRepo(db)

	for _, name := range []string{"gpt-4", "gpt-4o"} {
		if err := repo.Create(ctx, &models.ModelTemplateItem{ModelID: 1, Name: name}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}
	// 他模型的模板项不应被误删
	if err := repo.Create(ctx, &models.ModelTemplateItem{ModelID: 2, Name: "gpt-4"}); err != nil {
		t.Fatalf("Create other model item: %v", err)
	}

	deleted, err := repo.DeleteByModelIDUnscoped(ctx, 1)
	if err != nil {
		t.Fatalf("DeleteByModelIDUnscoped: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	var leftover int64
	if err := db.Unscoped().Model(&models.ModelTemplateItem{}).
		Where("model_id = ?", 1).Count(&leftover).Error; err != nil {
		t.Fatalf("count leftover: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("hard count after delete = %d, want 0", leftover)
	}

	var others int64
	if err := db.Model(&models.ModelTemplateItem{}).
		Where("model_id = ?", 2).Count(&others).Error; err != nil {
		t.Fatalf("count others: %v", err)
	}
	if others != 1 {
		t.Fatalf("other model items = %d, want 1", others)
	}

	// 硬删后可重建（unique index 不含 deleted_at）
	if err := repo.Create(ctx, &models.ModelTemplateItem{ModelID: 1, Name: "gpt-4"}); err != nil {
		t.Fatalf("recreate after hard delete: %v", err)
	}
}

func TestRepositories_RunInTx_Rollback(t *testing.T) {
	ctx := context.Background()
	db := newAssocTestDB(t)
	repos := New(db)

	errBoom := errors.New("boom")
	err := repos.RunInTx(ctx, func(txRepos *Repositories) error {
		if err := txRepos.ModelWithProvider.Create(ctx, &models.ModelWithProvider{
			ModelID: 1, ProviderID: 1, ProviderModel: "x",
		}); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("RunInTx err = %v, want boom", err)
	}

	all, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("rollback failed, still have %d rows", len(all))
	}
}

func TestRepositories_RunInTx_Commit(t *testing.T) {
	ctx := context.Background()
	repos := New(newAssocTestDB(t))

	err := repos.RunInTx(ctx, func(txRepos *Repositories) error {
		return txRepos.ModelTemplateItem.Create(ctx, &models.ModelTemplateItem{
			ModelID: 9, Name: "in-tx",
		})
	})
	if err != nil {
		t.Fatalf("RunInTx: %v", err)
	}

	count, err := repos.ModelTemplateItem.CountByModelIDAndName(ctx, 9, "in-tx")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("count after commit = %d, want 1", count)
	}
}

func TestDefault_LazyFromModelsDB(t *testing.T) {
	// 隔离 Default 全局状态
	SetDefault(nil)
	t.Cleanup(func() { SetDefault(nil) })

	db := newAssocTestDB(t)
	models.DB = db
	t.Cleanup(func() { models.DB = nil })

	r1 := Default()
	if r1 == nil {
		t.Fatal("Default() returned nil")
	}
	r2 := Default()
	if r1 != r2 {
		t.Error("Default() should return same instance")
	}

	explicit := New(db)
	SetDefault(explicit)
	if Default() != explicit {
		t.Error("SetDefault should replace Default instance")
	}
}
