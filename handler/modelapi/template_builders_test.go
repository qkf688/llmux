package modelapi

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestBuildModelTemplateResponse(t *testing.T) {
	model := models.Model{
		ID:   10,
		Name: "gpt-4.1",
	}
	associations := []models.ModelWithProvider{
		{ProviderModel: "gpt-4.1-mini"},
		{ProviderModel: "gpt-4.1-mini"},
	}
	manualItems := []models.ModelTemplateItem{
		{Name: "gpt-4.1-mini"},
		{Name: "gpt-4.1-pro"},
	}

	resp := buildModelTemplateResponse(model, associations, manualItems)
	if resp.ModelID != 10 || resp.ModelName != "gpt-4.1" {
		t.Fatalf("unexpected model metadata: %#v", resp)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items length = %d, want 3", len(resp.Items))
	}

	if resp.Items[0].Name != "gpt-4.1" {
		t.Fatalf("expected sorted first item to be model name, got %q", resp.Items[0].Name)
	}
	if len(resp.Items[0].Sources) != 1 || resp.Items[0].Sources[0] != "model_name" {
		t.Fatalf("unexpected sources for model name: %#v", resp.Items[0].Sources)
	}

	var miniSources []string
	for _, item := range resp.Items {
		if item.Name == "gpt-4.1-mini" {
			miniSources = item.Sources
			break
		}
	}
	if len(miniSources) != 2 {
		t.Fatalf("mini sources length = %d, want 2", len(miniSources))
	}
	if miniSources[0] != "association" || miniSources[1] != "manual" {
		t.Fatalf("unexpected mini sources ordering/value: %#v", miniSources)
	}
}
