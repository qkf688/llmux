package autoassoc

import (
	"context"
	"fmt"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func TestNewDefaultAssociation(t *testing.T) {
	assoc := NewDefaultAssociation(10, 20, "gpt-4.1", DefaultWeightFallback, 8)
	if assoc.ModelID != 10 || assoc.ProviderID != 20 || assoc.ProviderModel != "gpt-4.1" {
		t.Fatalf("unexpected identity: %#v", assoc)
	}
	if assoc.Priority != 8 || assoc.Weight != DefaultWeightFallback {
		t.Fatalf("priority/weight = %d/%d", assoc.Priority, assoc.Weight)
	}
	if assoc.ToolCall == nil || !*assoc.ToolCall {
		t.Fatal("ToolCall should be true")
	}
	if assoc.Status == nil || !*assoc.Status {
		t.Fatal("Status should be true")
	}
	if assoc.StructuredOutput == nil || *assoc.StructuredOutput {
		t.Fatal("StructuredOutput should be false")
	}
}

func TestAssociate_SkipsExistingBlacklistAndDisabledModel(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.Model.Create(ctx, &models.Model{ID: 1, Name: "gpt-4o", AutoAssociate: boolPtr(true)}); err != nil {
		t.Fatalf("create model1: %v", err)
	}
	if err := repos.Model.Create(ctx, &models.Model{ID: 2, Name: "blocked", AutoAssociate: boolPtr(false)}); err != nil {
		t.Fatalf("create model2: %v", err)
	}

	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "p1", Type: "openai",
		Config: `{"upstream_models":["gpt-4o","blocked","extra"]}`,
	}); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "p-black", Type: "openai", Blacklisted: boolPtr(true),
		Config: `{"upstream_models":["gpt-4o"]}`,
	}); err != nil {
		t.Fatalf("create p-black: %v", err)
	}

	providers, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil || len(providers) < 1 {
		t.Fatalf("list providers: %v len=%d", err, len(providers))
	}
	var p1ID uint
	for _, p := range providers {
		if p.Name == "p1" {
			p1ID = p.ID
		}
	}
	if p1ID == 0 {
		t.Fatal("p1 not found")
	}

	if err := repos.ModelWithProvider.Create(ctx, &models.ModelWithProvider{
		ModelID: 1, ProviderID: p1ID, ProviderModel: "gpt-4o", Weight: 1, Priority: 1,
	}); err != nil {
		t.Fatalf("seed existing: %v", err)
	}

	matcher := staticMatcher{
		"gpt-4o":  {1},
		"blocked": {2},
		"extra":   {1},
	}
	svc := newTestService(t, db, matcher)

	result, err := svc.Associate(ctx)
	if err != nil {
		t.Fatalf("Associate: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("added = %d, want 1 (only extra)", result.Success)
	}

	all, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("assoc count = %d, want 2", len(all))
	}

	var extra *models.ModelWithProvider
	for i := range all {
		if all[i].ProviderModel == "extra" {
			extra = &all[i]
		}
	}
	if extra == nil {
		t.Fatal("extra association missing")
	}
	if extra.Priority != DefaultPriorityFallback {
		t.Fatalf("extra.Priority = %d, want %d", extra.Priority, DefaultPriorityFallback)
	}
	if extra.Weight != DefaultWeightFallback {
		t.Fatalf("extra.Weight = %d, want %d", extra.Weight, DefaultWeightFallback)
	}

	previews, err := svc.PreviewAssociate(ctx)
	if err != nil {
		t.Fatalf("PreviewAssociate: %v", err)
	}
	// Preview bypasses Model.AutoAssociate: "blocked" (model 2, AutoAssociate=false)
	// was skipped by Associate but should appear in preview.
	if len(previews) != 1 {
		t.Fatalf("preview len = %d, want 1 (blocked, AutoAssociate=false)", len(previews))
	}
	if previews[0].ProviderModel != "blocked" {
		t.Fatalf("preview[0] = %q, want blocked", previews[0].ProviderModel)
	}
}

func TestAssociate_UsesPrioritySetting(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
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
	if err := repos.Setting.SetInt(ctx, models.SettingKeyAutoPriorityDecayDefault, 42); err != nil {
		t.Fatalf("set priority: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{"gpt-4o": {1}})
	result, err := svc.Associate(ctx)
	if err != nil {
		t.Fatalf("Associate: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("added = %d, want 1", result.Success)
	}
	all, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 || all[0].Priority != 42 {
		t.Fatalf("priority = %#v", all)
	}
}

// TestAssociate_UsesWeightSetting 验证 schema 真值经 DB → service 贯通到关联行的 Weight 字段。
// 与 TestAssociate_UsesPrioritySetting 对称：seed 一个不等于 DefaultWeightFallback 的值，
// 断言新建关联的 Weight 等于 seed 值——证明不是直接返回 Fallback 常量。
func TestAssociate_UsesWeightSetting(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
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
	if err := repos.Setting.SetInt(ctx, models.SettingKeyAutoWeightDecayDefault, 77); err != nil {
		t.Fatalf("set weight: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{"gpt-4o": {1}})
	result, err := svc.Associate(ctx)
	if err != nil {
		t.Fatalf("Associate: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("added = %d, want 1", result.Success)
	}
	all, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 || all[0].Weight != 77 {
		t.Fatalf("weight = %#v, want 77 (schema-seeded value, not fallback %d)", all, DefaultWeightFallback)
	}
}

// AssociateAll 绕过 Model.AutoAssociate 门控——手动关联不应被模型级开关拦截。
func TestAssociateAll_BypassesModelSwitch(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	// model with AutoAssociate=false
	if err := repos.Model.Create(ctx, &models.Model{ID: 1, Name: "gpt-4o", AutoAssociate: boolPtr(false)}); err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := repos.Provider.Create(ctx, &models.Provider{
		Name: "p1", Type: "openai",
		Config: `{"upstream_models":["gpt-4o"]}`,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{"gpt-4o": {1}})

	// Associate (automatic path) should skip model with AutoAssociate=false
	result, err := svc.Associate(ctx)
	if err != nil {
		t.Fatalf("Associate: %v", err)
	}
	if result.Success != 0 {
		t.Fatalf("Associate added = %d, want 0 (AutoAssociate=false)", result.Success)
	}

	// AssociateAll (manual path) should bypass the switch
	result, err = svc.AssociateAll(ctx)
	if err != nil {
		t.Fatalf("AssociateAll: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("AssociateAll added = %d, want 1 (bypassed switch)", result.Success)
	}
}

// 部分失败时不应返回 error，仅通过 Result.Failed 观测（Bug 30 回归）。
func TestAssociate_PartialFailureReturnsCountNoError(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.Model.Create(ctx, &models.Model{ID: 1, Name: "gpt-4o"}); err != nil {
		t.Fatalf("create model: %v", err)
	}
	// 两个 provider 都 serve "gpt-4o"，匹配同一 model → 两次 Create 都会失败
	for _, p := range []string{"p1", "p2"} {
		if err := repos.Provider.Create(ctx, &models.Provider{
			Name: p, Type: "openai",
			Config: `{"upstream_models":["gpt-4o"]}`,
		}); err != nil {
			t.Fatalf("create provider %s: %v", p, err)
		}
	}

	// 用包装 repo 使 Create 恒失败
	repos.ModelWithProvider = &failingCreateMWPRepo{
		ModelWithProviderRepo: repos.ModelWithProvider,
		err:                   fmt.Errorf("simulated create failure"),
	}
	svc := NewService(repos, func(
		_ []models.Model,
		_ []models.ModelWithProvider,
		_ []models.ModelTemplateItem,
	) NameMatcher {
		return staticMatcher{"gpt-4o": {1}}
	})

	result, err := svc.AssociateAll(ctx)
	if err != nil {
		t.Fatalf("AssociateAll should not return error on partial failure, got: %v", err)
	}
	if result.Success != 0 {
		t.Fatalf("Success = %d, want 0 (all creates failed)", result.Success)
	}
	if result.Failed != 2 {
		t.Fatalf("Failed = %d, want 2 (two providers, both failed)", result.Failed)
	}
}
