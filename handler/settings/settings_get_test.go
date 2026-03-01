package settings

import (
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestDefaultSettingsResponse(t *testing.T) {
	response := defaultSettingsResponse()

	if !response.StrictCapabilityMatch {
		t.Fatalf("expected StrictCapabilityMatch default to be true")
	}
	if response.AutoWeightDecayDefault != 100 {
		t.Fatalf("expected AutoWeightDecayDefault default to be 100, got %d", response.AutoWeightDecayDefault)
	}
	if response.ModelSyncInterval != 12 {
		t.Fatalf("expected ModelSyncInterval default to be 12, got %d", response.ModelSyncInterval)
	}
	if response.ReasoningEffortDefaultValue != "low" {
		t.Fatalf("expected ReasoningEffortDefaultValue default to be low, got %s", response.ReasoningEffortDefaultValue)
	}
}

func TestApplySettingToResponse(t *testing.T) {
	response := defaultSettingsResponse()

	settings := []models.Setting{
		{Key: models.SettingKeyAutoWeightDecay, Value: "true"},
		{Key: models.SettingKeyAutoWeightDecayDefault, Value: "88"},
		{Key: models.SettingKeyModelSyncFilterRules, Value: `["gpt","claude"]`},
		{Key: models.SettingKeyReasoningEffortDefaultValue, Value: "high"},
	}

	for _, setting := range settings {
		applySettingToResponse(&response, setting)
	}

	if !response.AutoWeightDecay {
		t.Fatalf("expected AutoWeightDecay to be true")
	}
	if response.AutoWeightDecayDefault != 88 {
		t.Fatalf("expected AutoWeightDecayDefault to be 88, got %d", response.AutoWeightDecayDefault)
	}
	if len(response.ModelSyncFilterRules) != 2 {
		t.Fatalf("expected ModelSyncFilterRules length to be 2, got %d", len(response.ModelSyncFilterRules))
	}
	if response.ModelSyncFilterRules[0] != "gpt" || response.ModelSyncFilterRules[1] != "claude" {
		t.Fatalf("unexpected ModelSyncFilterRules value: %#v", response.ModelSyncFilterRules)
	}
	if response.ReasoningEffortDefaultValue != "high" {
		t.Fatalf("expected ReasoningEffortDefaultValue to be high, got %s", response.ReasoningEffortDefaultValue)
	}
}

func TestApplySettingToResponseKeepsDefaultsOnInvalidValues(t *testing.T) {
	response := defaultSettingsResponse()

	applySettingToResponse(&response, models.Setting{
		Key:   models.SettingKeyAutoWeightDecayDefault,
		Value: "invalid-int",
	})
	applySettingToResponse(&response, models.Setting{
		Key:   models.SettingKeyTemplateFuzzyMatchSeparators,
		Value: "invalid-json",
	})

	if response.AutoWeightDecayDefault != 100 {
		t.Fatalf("expected AutoWeightDecayDefault to keep default 100, got %d", response.AutoWeightDecayDefault)
	}
	if len(response.TemplateFuzzyMatchSeparators) != 2 {
		t.Fatalf("expected TemplateFuzzyMatchSeparators to keep default length 2, got %d", len(response.TemplateFuzzyMatchSeparators))
	}
	if response.TemplateFuzzyMatchSeparators[0] != ":" || response.TemplateFuzzyMatchSeparators[1] != "-" {
		t.Fatalf("unexpected TemplateFuzzyMatchSeparators value: %#v", response.TemplateFuzzyMatchSeparators)
	}
}
