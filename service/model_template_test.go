package service

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestTemplateIndexMatch_UnionAndCaseSensitive(t *testing.T) {
	allModels := []models.Model{
		{ID: 1, Name: "gpt-4o"},
		{ID: 2, Name: "claude"},
	}
	allAssociations := []models.ModelWithProvider{
		{ModelID: 1, ProviderModel: "alias-x"},
	}
	manual := []models.ModelTemplateItem{
		{ModelID: 2, Name: "alias-x"},
		{ModelID: 2, Name: "manual-only"},
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
