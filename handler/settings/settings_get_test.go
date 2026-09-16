package settings

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestDefaultSettingsResponse(t *testing.T) {
	response, err := buildSettingsResponse(nil)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}

	if !response.StrictCapabilityMatch {
		t.Fatalf("expected StrictCapabilityMatch default to be true")
	}
	if response.AutoWeightDecayDefault != 100 {
		t.Fatalf("expected AutoWeightDecayDefault default to be 100, got %d", response.AutoWeightDecayDefault)
	}
	if response.ModelSyncInterval != 12 {
		t.Fatalf("expected ModelSyncInterval default to be 12, got %d", response.ModelSyncInterval)
	}
	if response.CredHealthCooldown429Sec != 60 {
		t.Fatalf("expected CredHealthCooldown429Sec default to be 60, got %d", response.CredHealthCooldown429Sec)
	}
	if response.CredHealthCooldownServerSec != 60 {
		t.Fatalf("expected CredHealthCooldownServerSec default to be 60, got %d", response.CredHealthCooldownServerSec)
	}
	if response.CredHealthAuthFailThreshold != 3 {
		t.Fatalf("expected CredHealthAuthFailThreshold default to be 3, got %d", response.CredHealthAuthFailThreshold)
	}
	if response.CredHealthProbeIntervalSec != 60 {
		t.Fatalf("expected CredHealthProbeIntervalSec default to be 60, got %d", response.CredHealthProbeIntervalSec)
	}
	if response.ReasoningEffortDefaultValue != "low" {
		t.Fatalf("expected ReasoningEffortDefaultValue default to be low, got %s", response.ReasoningEffortDefaultValue)
	}
	if response.LogRawRequestResponseErrorsOnly {
		t.Fatalf("expected LogRawRequestResponseErrorsOnly default to be false")
	}
	// Stage 2-C：上游最大响应体默认 32 MiB（文生图 base64 合法场景 ×4 余量，见 schema 注释）。
	if response.UpstreamMaxResponseMiB != 32 {
		t.Fatalf("expected UpstreamMaxResponseMiB default to be 32, got %d", response.UpstreamMaxResponseMiB)
	}
	// Stage 2-D：原始采集字段上限默认 256 KiB（普通响应完整、异常大响应截尾）。
	if response.RawLogFieldMaxKiB != 256 {
		t.Fatalf("expected RawLogFieldMaxKiB default to be 256, got %d", response.RawLogFieldMaxKiB)
	}
}

func TestApplySettingToResponse(t *testing.T) {
	settings := []models.Setting{
		{Key: models.SettingKeyAutoWeightDecay, Value: "true"},
		{Key: models.SettingKeyAutoWeightDecayDefault, Value: "88"},
		{Key: models.SettingKeyLogRawRequestResponseErrorsOnly, Value: "true"},
		{Key: models.SettingKeyModelSyncFilterRules, Value: `["gpt","claude"]`},
		{Key: models.SettingKeyReasoningEffortDefaultValue, Value: "high"},
	}

	response, err := buildSettingsResponse(settings)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
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
	if !response.LogRawRequestResponseErrorsOnly {
		t.Fatalf("expected LogRawRequestResponseErrorsOnly to be true")
	}
}

func TestApplySettingToResponseKeepsDefaultsOnInvalidValues(t *testing.T) {
	settings := []models.Setting{
		{Key: models.SettingKeyAutoWeightDecayDefault, Value: "invalid-int"},
		{Key: models.SettingKeyTemplateFuzzyMatchSeparators, Value: "invalid-json"},
	}

	response, err := buildSettingsResponse(settings)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}

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
