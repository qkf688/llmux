package repository

import (
	"context"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
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
