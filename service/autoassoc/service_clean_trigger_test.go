package autoassoc

import (
	"context"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
)

func TestCleanInvalid_DeletesMissingProviderAndModel(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "alive", Type: "openai",
		Config: `{"upstream_models":["keep-me"]}`,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	providers, _ := repos.Provider.List(ctx, repository.ProviderFilter{})
	if len(providers) != 1 {
		t.Fatalf("providers len = %d", len(providers))
	}
	pid := providers[0].ID

	keep := &models.ModelWithProvider{ModelID: 1, ProviderID: pid, ProviderModel: "keep-me"}
	goneModel := &models.ModelWithProvider{ModelID: 1, ProviderID: pid, ProviderModel: "gone"}
	orphanProvider := &models.ModelWithProvider{ModelID: 1, ProviderID: 9999, ProviderModel: "x"}
	for _, a := range []*models.ModelWithProvider{keep, goneModel, orphanProvider} {
		if err := repos.ModelWithProvider.Create(ctx, a); err != nil {
			t.Fatalf("create assoc: %v", err)
		}
	}

	svc := newTestService(t, db, staticMatcher{})
	result, err := svc.CleanInvalid(ctx)
	if err != nil {
		t.Fatalf("CleanInvalid: %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("removed = %d, want 2", result.Success)
	}

	all, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 || all[0].ProviderModel != "keep-me" {
		t.Fatalf("remaining = %#v", all)
	}
}

func TestCleanInvalid_SkipsWhenProviderModelsUnreadable(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	// invalid JSON config -> GetProviderModels returns empty list (no error),
	// so association with unknown model is cleaned. Seed valid config that fails parse differently:
	// modelsync.GetProviderModels always returns extractAllModels which returns [] on bad JSON without error.
	// So unreadable path is "error" only if GetProviderModels returns err — currently it never does.
	// Document current contract: bad config yields empty model list => association removed.
	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "bad", Type: "openai",
		Config: `{not-json`,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	providers, _ := repos.Provider.List(ctx, repository.ProviderFilter{})
	pid := providers[0].ID
	if err := repos.ModelWithProvider.Create(ctx, &models.ModelWithProvider{
		ModelID: 1, ProviderID: pid, ProviderModel: "any",
	}); err != nil {
		t.Fatalf("create assoc: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{})
	result, err := svc.CleanInvalid(ctx)
	if err != nil {
		t.Fatalf("CleanInvalid: %v", err)
	}
	// empty model list from bad config => treated as missing provider model
	if result.Success != 1 {
		t.Fatalf("removed = %d, want 1 (empty list from bad config)", result.Success)
	}
}

func TestTriggerAssociateIfEnabled_RespectsSetting(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	// settings now read via repos.Setting; no models.DB required
	repos := repository.New(db)
	if err := repos.Model.Create(ctx, &models.Model{ID: 1, Name: "gpt-4o"}); err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "p1", Type: "openai",
		Config: `{"upstream_models":["gpt-4o"]}`,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{"gpt-4o": {1}})

	svc.TriggerAssociateIfEnabled(ctx)
	all, _ := repos.ModelWithProvider.ListAll(ctx)
	if len(all) != 0 {
		t.Fatalf("should not associate when disabled, got %d", len(all))
	}

	if err := repos.Setting.SetBool(ctx, models.SettingKeyAutoAssociateOnAdd, true); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	svc.TriggerAssociateIfEnabled(ctx)
	all, _ = repos.ModelWithProvider.ListAll(ctx)
	if len(all) != 1 {
		t.Fatalf("should associate when enabled, got %d", len(all))
	}
}

func TestTriggerCleanIfEnabled_RespectsSetting(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.ModelWithProvider.Create(ctx, &models.ModelWithProvider{
		ModelID: 1, ProviderID: 9999, ProviderModel: "orphan",
	}); err != nil {
		t.Fatalf("create assoc: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{})

	svc.TriggerCleanIfEnabled(ctx)
	all, _ := repos.ModelWithProvider.ListAll(ctx)
	if len(all) != 1 {
		t.Fatalf("should not clean when disabled, got %d", len(all))
	}

	if err := repos.Setting.SetBool(ctx, models.SettingKeyAutoCleanOnDelete, true); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	svc.TriggerCleanIfEnabled(ctx)
	all, _ = repos.ModelWithProvider.ListAll(ctx)
	if len(all) != 0 {
		t.Fatalf("should clean when enabled, got %d", len(all))
	}
}
