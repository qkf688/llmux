package service

import (
	"testing"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func TestTemplateIndexMatch_UnionAndCaseSensitive(t *testing.T) {
	allModels := []models.Model{
		{Model: gorm.Model{ID: 1}, Name: "gpt-4o"},
		{Model: gorm.Model{ID: 2}, Name: "claude"},
	}
	allAssociations := []models.ModelWithProvider{
		{Model: gorm.Model{ID: 10}, ModelID: 1, ProviderModel: "alias-x"},
	}
	manual := []models.ModelTemplateItem{
		{Model: gorm.Model{ID: 20}, ModelID: 2, Name: "alias-x"},
		{Model: gorm.Model{ID: 21}, ModelID: 2, Name: "manual-only"},
	}

	index := BuildTemplateIndexFromData(allModels, allAssociations, manual)

	got := index.Match("alias-x")
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected match [1 2], got %v", got)
	}

	got = index.Match("gpt-4o")
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected match [1], got %v", got)
	}

	got = index.Match("ALIAS-X")
	if len(got) != 0 {
		t.Fatalf("expected case-sensitive no match, got %v", got)
	}
}

