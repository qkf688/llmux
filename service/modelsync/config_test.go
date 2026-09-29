package modelsync

import (
	"context"
	"reflect"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func TestMatchesAnyRule(t *testing.T) {
	rules := []string{"gpt", "claude"}
	if !matchesAnyRule("gpt-4.1", rules) {
		t.Fatal("expected rule match for gpt-4.1")
	}
	if matchesAnyRule("gemini-2.0", rules) {
		t.Fatal("did not expect rule match for gemini-2.0")
	}
}

func TestGetProviderModels_FromGroupWhitelist(t *testing.T) {
	db := testutil.New(t, &models.Provider{}, &models.KeyGroup{})

	provider := models.Provider{
		Name:   "p",
		Type:   "openai",
		Config: `{"custom_models":["c1"],"upstream_models":["ignored"]}`,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := db.Create(&models.KeyGroup{ProviderID: provider.ID, Name: "g", Models: "u1,u2"}).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	got, err := GetProviderModels(context.Background(), provider, repository.New(db))
	if err != nil {
		t.Fatalf("GetProviderModels() error = %v", err)
	}
	expected := []string{"u1", "u2", "c1"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("GetProviderModels() = %v, want %v", got, expected)
	}
}
