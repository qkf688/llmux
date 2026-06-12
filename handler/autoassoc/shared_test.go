package autoassoc

import (
	"testing"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func TestBuildAssociationKey(t *testing.T) {
	if got, want := buildAssociationKey(1, 2, "gpt-4.1"), "1_2_gpt-4.1"; got != want {
		t.Fatalf("buildAssociationKey() = %q, want %q", got, want)
	}
}

func TestBuildExistingAssociationMap(t *testing.T) {
	associations := []models.ModelWithProvider{
		{ModelID: 1, ProviderID: 2, ProviderModel: "gpt-4.1"},
		{ModelID: 3, ProviderID: 4, ProviderModel: "claude-4"},
	}

	existingMap := buildExistingAssociationMap(associations)
	if !existingMap["1_2_gpt-4.1"] {
		t.Fatalf("missing key for first association")
	}
	if !existingMap["3_4_claude-4"] {
		t.Fatalf("missing key for second association")
	}
	if existingMap["1_2_unknown"] {
		t.Fatalf("unexpected key exists")
	}
}

func TestIndexModelsByID(t *testing.T) {
	allModels := []models.Model{
		{ID: 10, Name: "gpt-4.1"},
		{ID: 11, Name: "claude-4"},
	}

	modelByID := indexModelsByID(allModels)
	if modelByID[10].Name != "gpt-4.1" {
		t.Fatalf("modelByID[10].Name = %q", modelByID[10].Name)
	}
	if modelByID[11].Name != "claude-4" {
		t.Fatalf("modelByID[11].Name = %q", modelByID[11].Name)
	}
}

func TestIndexProvidersByID(t *testing.T) {
	allProviders := []models.Provider{
		{Model: gorm.Model{ID: 20}, Name: "openai"},
		{Model: gorm.Model{ID: 21}, Name: "anthropic"},
	}

	providerByID := indexProvidersByID(allProviders)
	if providerByID[20] == nil || providerByID[20].Name != "openai" {
		t.Fatalf("providerByID[20] = %#v", providerByID[20])
	}
	if providerByID[21] == nil || providerByID[21].Name != "anthropic" {
		t.Fatalf("providerByID[21] = %#v", providerByID[21])
	}
}

func TestProviderModelExists(t *testing.T) {
	providerModels := []string{"gpt-4.1", "claude-4"}
	if !providerModelExists(providerModels, "claude-4") {
		t.Fatalf("expected providerModelExists to return true")
	}
	if providerModelExists(providerModels, "gemini-2.0") {
		t.Fatalf("expected providerModelExists to return false")
	}
}

func TestIsProviderBlacklisted(t *testing.T) {
	trueVal := true
	falseVal := false
	tests := []struct {
		name     string
		provider models.Provider
		want     bool
	}{
		{
			name: "nil blacklisted",
			provider: models.Provider{
				Name: "openai",
			},
			want: false,
		},
		{
			name: "blacklisted true",
			provider: models.Provider{
				Name:        "openai",
				Blacklisted: &trueVal,
			},
			want: true,
		},
		{
			name: "blacklisted false",
			provider: models.Provider{
				Name:        "openai",
				Blacklisted: &falseVal,
			},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isProviderBlacklisted(tc.provider); got != tc.want {
				t.Fatalf("isProviderBlacklisted() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewDefaultAssociation(t *testing.T) {
	assoc := newDefaultAssociation(10, 20, "gpt-4.1", 8)

	if assoc.ModelID != 10 || assoc.ProviderID != 20 || assoc.ProviderModel != "gpt-4.1" {
		t.Fatalf("unexpected identity fields: %#v", assoc)
	}
	if assoc.Priority != 8 || assoc.Weight != 5 {
		t.Fatalf("unexpected priority/weight: priority=%d weight=%d", assoc.Priority, assoc.Weight)
	}
	if assoc.ToolCall == nil || !*assoc.ToolCall {
		t.Fatalf("ToolCall should default to true")
	}
	if assoc.Status == nil || !*assoc.Status {
		t.Fatalf("Status should default to true")
	}
	if assoc.StructuredOutput == nil || *assoc.StructuredOutput {
		t.Fatalf("StructuredOutput should default to false")
	}
	if assoc.Image == nil || *assoc.Image {
		t.Fatalf("Image should default to false")
	}
	if assoc.WithHeader == nil || *assoc.WithHeader {
		t.Fatalf("WithHeader should default to false")
	}
	if assoc.CustomerHeaders == nil {
		t.Fatalf("CustomerHeaders should be initialized")
	}
}
