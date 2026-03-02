package modelsync

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestDropCustomModels(t *testing.T) {
	config := `{"api_key":"k","custom_models":["c1"],"upstream_models":["u1"],"base_url":"https://example.com"}`

	updated, err := dropCustomModels(config)
	if err != nil {
		t.Fatalf("dropCustomModels() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(updated), &parsed); err != nil {
		t.Fatalf("failed to parse updated config: %v", err)
	}

	if _, exists := parsed["custom_models"]; exists {
		t.Fatal("custom_models should be removed")
	}
	if _, exists := parsed["upstream_models"]; exists {
		t.Fatal("upstream_models should be removed")
	}
	if parsed["api_key"] != "k" {
		t.Fatalf("api_key should remain unchanged, got %v", parsed["api_key"])
	}
}

func TestExtractAllModels(t *testing.T) {
	config := `{"upstream_models":["u1","u2"],"custom_models":["c1"]}`
	expected := []string{"u1", "u2", "c1"}

	got := extractAllModels(config)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("extractAllModels() = %v, want %v", got, expected)
	}
}

func TestExtractUpstreamModels(t *testing.T) {
	config := `{"upstream_models":["u1","u2"],"custom_models":["c1"]}`
	expected := []string{"u1", "u2"}

	got := extractUpstreamModels(config)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("extractUpstreamModels() = %v, want %v", got, expected)
	}
}

func TestBuildConfigWithAllModels(t *testing.T) {
	original := `{"api_key":"k","upstream_models":["old"],"custom_models":["c1"]}`
	updated := buildConfigWithAllModels(original, []string{"n1", "n2"})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(updated), &parsed); err != nil {
		t.Fatalf("failed to parse updated config: %v", err)
	}

	upstreamAny, ok := parsed["upstream_models"].([]interface{})
	if !ok {
		t.Fatalf("upstream_models should exist and be array, got %T", parsed["upstream_models"])
	}

	got := make([]string, 0, len(upstreamAny))
	for _, item := range upstreamAny {
		got = append(got, item.(string))
	}
	expected := []string{"n1", "n2"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("upstream_models = %v, want %v", got, expected)
	}

	customAny, ok := parsed["custom_models"].([]interface{})
	if !ok || len(customAny) != 1 || customAny[0].(string) != "c1" {
		t.Fatalf("custom_models should remain unchanged, got %v", parsed["custom_models"])
	}
}

func TestMatchesAnyRule(t *testing.T) {
	rules := []string{"gpt", "claude"}
	if !matchesAnyRule("gpt-4.1", rules) {
		t.Fatal("expected rule match for gpt-4.1")
	}
	if matchesAnyRule("gemini-2.0", rules) {
		t.Fatal("did not expect rule match for gemini-2.0")
	}
}

func TestGetProviderModels(t *testing.T) {
	provider := models.Provider{
		Config: `{"upstream_models":["u1"],"custom_models":["c1","c2"]}`,
	}

	got, err := GetProviderModels(context.Background(), provider)
	if err != nil {
		t.Fatalf("GetProviderModels() error = %v", err)
	}

	expected := []string{"u1", "c1", "c2"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("GetProviderModels() = %v, want %v", got, expected)
	}
}
