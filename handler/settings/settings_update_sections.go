package settings

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/atopos31/llmio/models"
)

func updateStrictAndWeightSettings(ctx context.Context, req *UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyStrictCapabilityMatch, req.StrictCapabilityMatch); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyAutoWeightDecay, req.AutoWeightDecay); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoWeightDecayDefault, req.AutoWeightDecayDefault); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoWeightDecayStep, req.AutoWeightDecayStep); err != nil {
		return err
	}

	if req.AutoSuccessIncrease {
		if req.AutoWeightIncreaseStep < 1 {
			req.AutoWeightIncreaseStep = 1
		}
		if req.AutoWeightIncreaseMax < 1 {
			req.AutoWeightIncreaseMax = 100
		}
	}

	if err := updateBoolSetting(ctx, models.SettingKeyAutoSuccessIncrease, req.AutoSuccessIncrease); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoWeightIncreaseStep, req.AutoWeightIncreaseStep); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoWeightIncreaseMax, req.AutoWeightIncreaseMax); err != nil {
		return err
	}

	return nil
}

func updatePriorityAndFailureSettings(ctx context.Context, req *UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyAutoPriorityDecay, req.AutoPriorityDecay); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoPriorityDecayDefault, req.AutoPriorityDecayDefault); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoPriorityDecayStep, req.AutoPriorityDecayStep); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoPriorityDecayThreshold, req.AutoPriorityDecayThreshold); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyAutoPriorityDecayDisableEnabled, req.AutoPriorityDecayDisableEnabled); err != nil {
		return err
	}

	if req.AutoPriorityIncreaseStep < 1 {
		req.AutoPriorityIncreaseStep = 1
	}
	if req.AutoPriorityIncreaseMax < 0 {
		req.AutoPriorityIncreaseMax = 100
	}

	if err := updateIntSetting(ctx, models.SettingKeyAutoPriorityIncreaseStep, req.AutoPriorityIncreaseStep); err != nil {
		return err
	}
	if err := updateIntSetting(ctx, models.SettingKeyAutoPriorityIncreaseMax, req.AutoPriorityIncreaseMax); err != nil {
		return err
	}

	if req.ConsecutiveFailureThreshold < 1 {
		req.ConsecutiveFailureThreshold = 3
	}

	if err := updateIntSetting(ctx, models.SettingKeyConsecutiveFailureThreshold, req.ConsecutiveFailureThreshold); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyConsecutiveFailureDisableEnabled, req.ConsecutiveFailureDisableEnabled); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyHealthCheckCountAsSuccess, req.CountHealthCheckAsSuccess); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyHealthCheckCountAsFailure, req.CountHealthCheckAsFailure); err != nil {
		return err
	}

	return nil
}

func updateLogSettings(ctx context.Context, req UpdateSettingsRequest) error {
	if err := updateIntSetting(ctx, models.SettingKeyLogRetentionCount, req.LogRetentionCount); err != nil {
		return err
	}

	logRawOptionsJSON, err := json.Marshal(req.LogRawRequestResponse)
	if err != nil {
		return newDirectClientError("Failed to marshal log raw options: " + err.Error())
	}
	if err := updateStringSetting(ctx, models.SettingKeyLogRawRequestResponse, string(logRawOptionsJSON)); err != nil {
		return err
	}

	if err := updateBoolSetting(ctx, models.SettingKeyDisableAllLogs, req.DisableAllLogs); err != nil {
		return err
	}

	return nil
}

func updatePerformanceSettings(ctx context.Context, req UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyDisablePerformanceTracking, req.DisablePerformanceTracking); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyDisableTokenCounting, req.DisableTokenCounting); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyEnableRequestTrace, req.EnableRequestTrace); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyStripResponseHeaders, req.StripResponseHeaders); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyEnableFormatConversion, req.EnableFormatConversion); err != nil {
		return err
	}

	return nil
}

func updateAssociationSettings(ctx context.Context, req UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyAutoAssociateOnAdd, req.AutoAssociateOnAdd); err != nil {
		return err
	}
	if err := updateBoolSetting(ctx, models.SettingKeyAutoCleanOnDelete, req.AutoCleanOnDelete); err != nil {
		return err
	}

	oldAutoSaveValue := GetSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate)
	if err := updateBoolSetting(ctx, models.SettingKeyAutoSaveTemplateOnAssociate, req.AutoSaveTemplateOnAssociate); err != nil {
		return err
	}

	if !oldAutoSaveValue && req.AutoSaveTemplateOnAssociate {
		triggerBatchImportForAutoSave()
		slog.Info("triggered batch import of existing associations")
	}

	return nil
}

func updateModelSyncSettings(ctx context.Context, req *UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyModelSyncEnabled, req.ModelSyncEnabled); err != nil {
		return err
	}

	if req.ModelSyncInterval < 1 {
		req.ModelSyncInterval = 12
	}
	if err := updateIntSetting(ctx, models.SettingKeyModelSyncInterval, req.ModelSyncInterval); err != nil {
		return err
	}

	if req.ModelSyncLogRetentionCount < 0 {
		req.ModelSyncLogRetentionCount = 100
	}
	if err := updateIntSetting(ctx, models.SettingKeyModelSyncLogRetentionCount, req.ModelSyncLogRetentionCount); err != nil {
		return err
	}

	if req.ModelSyncLogRetentionDays < 0 {
		req.ModelSyncLogRetentionDays = 7
	}
	if err := updateIntSetting(ctx, models.SettingKeyModelSyncLogRetentionDays, req.ModelSyncLogRetentionDays); err != nil {
		return err
	}

	filterRulesJSON, err := json.Marshal(req.ModelSyncFilterRules)
	if err != nil {
		return newDirectClientError("Failed to marshal filter rules: " + err.Error())
	}
	if err := updateStringSetting(ctx, models.SettingKeyModelSyncFilterRules, string(filterRulesJSON)); err != nil {
		return err
	}

	return nil
}

func updateTemplateFuzzyMatchSettings(ctx context.Context, req UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyTemplateFuzzyMatchEnabled, req.TemplateFuzzyMatchEnabled); err != nil {
		return err
	}

	separatorsJSON, err := json.Marshal(req.TemplateFuzzyMatchSeparators)
	if err != nil {
		return newDirectClientError("Failed to marshal separators: " + err.Error())
	}
	if err := updateStringSetting(ctx, models.SettingKeyTemplateFuzzyMatchSeparators, string(separatorsJSON)); err != nil {
		return err
	}

	suffixesJSON, err := json.Marshal(req.TemplateFuzzyMatchSuffixes)
	if err != nil {
		return newDirectClientError("Failed to marshal suffixes: " + err.Error())
	}
	if err := updateStringSetting(ctx, models.SettingKeyTemplateFuzzyMatchSuffixes, string(suffixesJSON)); err != nil {
		return err
	}

	return nil
}

func updateReasoningEffortSettings(ctx context.Context, req *UpdateSettingsRequest) error {
	if err := updateBoolSetting(ctx, models.SettingKeyReasoningEffortMappingEnabled, req.ReasoningEffortMappingEnabled); err != nil {
		return err
	}

	if req.ReasoningEffortDefaultValue != "low" &&
		req.ReasoningEffortDefaultValue != "medium" &&
		req.ReasoningEffortDefaultValue != "high" {
		req.ReasoningEffortDefaultValue = "low"
	}
	if err := updateStringSetting(ctx, models.SettingKeyReasoningEffortDefaultValue, req.ReasoningEffortDefaultValue); err != nil {
		return err
	}

	return nil
}
