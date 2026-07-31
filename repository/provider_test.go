package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Provider{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestProviderRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewProviderRepo(newTestDB(t))

	trueVal := true
	falseVal := false
	provider := &models.Provider{
		Name:               "test-provider",
		Type:               "openai",
		Config:             `{}`,
		ModelEndpoint:      &trueVal,
		ModelFilterEnabled: &falseVal,
	}

	if err := repo.Create(ctx, provider); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if provider.ID == 0 {
		t.Fatal("provider ID should be set after create")
	}

	got, err := repo.Get(ctx, provider.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Name != provider.Name {
		t.Errorf("Get Name = %q, want %q", got.Name, provider.Name)
	}

	exists, err := repo.ExistsByName(ctx, provider.Name)
	if err != nil {
		t.Fatalf("ExistsByName error: %v", err)
	}
	if !exists {
		t.Error("ExistsByName should return true")
	}

	provider.Name = "updated-provider"
	if err := repo.Update(ctx, provider.ID, provider); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	got, _ = repo.GetByName(ctx, "updated-provider")
	if got == nil || got.Name != "updated-provider" {
		t.Fatal("GetByName failed after update")
	}

	affected, err := repo.Delete(ctx, provider.ID)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if affected != 1 {
		t.Errorf("Delete RowsAffected = %d, want 1", affected)
	}
}

func TestProviderRepo_List_FilterByType(t *testing.T) {
	ctx := context.Background()
	repo := NewProviderRepo(newTestDB(t))

	trueVal := true
	for _, p := range []*models.Provider{
		{Name: "openai-provider", Type: "openai", Config: `{}`, ModelEndpoint: &trueVal},
		{Name: "anthropic-provider", Type: "anthropic", Config: `{}`, ModelEndpoint: &trueVal},
	} {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	providers, err := repo.List(ctx, ProviderFilter{Type: "openai"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("List returned %d providers, want 1", len(providers))
	}
	if providers[0].Type != "openai" {
		t.Errorf("List Type = %q, want openai", providers[0].Type)
	}
}

func TestProviderRepo_List_FilterByModelEndpoint_NullRowsMatchTrue(t *testing.T) {
	// 历史行的 ModelEndpoint 是 NULL（字段无 default tag），语义为「默认 true」。
	// 修复前 `model_endpoint = ?` 会漏掉所有 NULL 行；修复后 `IS NULL OR = ?` 应命中 NULL + =true 两类行。
	// 全仓 4 处 modelsync caller 都只传 &true 拉可同步供应商；&false 的语义没有 caller 也没定义，不在断言范围。
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewProviderRepo(db)

	trueVal := true
	providers := []*models.Provider{
		{Name: "p-true", Type: "openai", Config: `{}`, ModelEndpoint: &trueVal},
		{Name: "p-null", Type: "openai", Config: `{}`}, // ModelEndpoint 未设置 → NULL
	}
	for _, p := range providers {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	listTrue, err := repo.List(ctx, ProviderFilter{ModelEndpoint: &trueVal})
	if err != nil {
		t.Fatalf("List true error: %v", err)
	}
	if len(listTrue) != 2 {
		t.Fatalf("ModelEndpoint=true 命中 %d 行 (p-true + p-null)，want 2", len(listTrue))
	}
	gotNames := map[string]bool{}
	for _, p := range listTrue {
		gotNames[p.Name] = true
	}
	if !gotNames["p-true"] || !gotNames["p-null"] {
		t.Errorf("命中集合 = %v, want 含 p-true 与 p-null", gotNames)
	}
}

func TestProviderRepo_UpdateBlacklist(t *testing.T) {
	ctx := context.Background()
	repo := NewProviderRepo(newTestDB(t))

	trueVal := true
	falseVal := false
	providers := []*models.Provider{
		{Name: "p1", Type: "openai", Config: `{}`, Blacklisted: &trueVal, ModelEndpoint: &trueVal},
		{Name: "p2", Type: "openai", Config: `{}`, Blacklisted: &falseVal, ModelEndpoint: &trueVal},
		{Name: "p3", Type: "openai", Config: `{}`, Blacklisted: &trueVal, ModelEndpoint: &trueVal},
	}
	for _, p := range providers {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	affected, err := repo.UpdateBlacklist(ctx, []uint{providers[1].ID})
	if err != nil {
		t.Fatalf("UpdateBlacklist error: %v", err)
	}
	if affected != 1 {
		t.Errorf("UpdateBlacklist RowsAffected = %d, want 1", affected)
	}

	list, err := repo.List(ctx, ProviderFilter{Blacklisted: &trueVal})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("blacklisted count = %d, want 1", len(list))
	}
	if list[0].ID != providers[1].ID {
		t.Errorf("blacklisted ID = %d, want %d", list[0].ID, providers[1].ID)
	}
}
