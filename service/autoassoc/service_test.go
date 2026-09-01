package autoassoc

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

type staticMatcher map[string][]uint

func (m staticMatcher) Match(name string) []uint {
	return m[name]
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:autoassoc_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Model{},
		&models.Provider{},
		&models.ModelWithProvider{},
		&models.ModelTemplateItem{},
		&models.Setting{},
		&models.KeyGroup{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedProviderModels 为供应商写入分组白名单（组织级目录来源，替代旧 Config.upstream_models）。
func seedProviderModels(t *testing.T, repos *repository.Repositories, providerID uint, modelsCSV string) {
	t.Helper()
	ctx := context.Background()
	if err := repos.KeyGroup.Create(ctx, &models.KeyGroup{
		ProviderID: providerID,
		Name:       "default",
		Weight:     1,
		Models:     modelsCSV,
	}); err != nil {
		t.Fatalf("seed key group models: %v", err)
	}
}

func newTestService(t *testing.T, db *gorm.DB, matcher NameMatcher) *Service {
	t.Helper()
	repos := repository.New(db)
	return NewService(repos, func(
		_ []models.Model,
		_ []models.ModelWithProvider,
		_ []models.ModelTemplateItem,
	) NameMatcher {
		return matcher
	})
}

func boolPtr(v bool) *bool { return &v }

func TestSharedHelpers(t *testing.T) {
	if got := buildAssociationKey(1, 2, "m"); got != "1_2_m" {
		t.Fatalf("key = %q", got)
	}
	if !providerModelExists([]string{"a", "b"}, "b") {
		t.Fatal("expected exists")
	}
	if providerModelExists([]string{"a"}, "z") {
		t.Fatal("expected not exists")
	}
	if isProviderBlacklisted(models.Provider{}) {
		t.Fatal("nil blacklisted should be false")
	}
	if !isProviderBlacklisted(models.Provider{Blacklisted: boolPtr(true)}) {
		t.Fatal("true blacklisted")
	}
	if !allowsAutoAssociate(models.Model{}) {
		t.Fatal("nil AutoAssociate should allow")
	}
	if allowsAutoAssociate(models.Model{AutoAssociate: boolPtr(false)}) {
		t.Fatal("false should deny")
	}
}

// failingCreateMWPRepo 包装 ModelWithProviderRepo，使 Create 恒失败。用于部分失败测试。
type failingCreateMWPRepo struct {
	repository.ModelWithProviderRepo
	err error
}

func (r *failingCreateMWPRepo) Create(ctx context.Context, assoc *models.ModelWithProvider) error {
	return r.err
}
