import { useCallback } from "react";
import { toast } from "sonner";
import { updateHealthCheckSettings, type HealthCheckSettings } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import type { HealthCheckSettingsProps } from "../types";
import { healthCheckSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import { useSettingsEditorSync } from "../../hooks/use-settings-editor-sync";

export function useHealthCheckSettingsForm({
  healthCheckSettings,
  onHealthCheckSettingsChange,
}: HealthCheckSettingsProps) {
  const {
    saving,
    localSettings,
    hasChanges,
    setSaving,
    syncFromServerSettings,
    updateLocalSettings: updateLocalSettingsInternal,
    markSaved,
    resetTransient,
  } = useSettingsStore(healthCheckSettingsEditorStore, (state) => state);

  useSettingsEditorSync({
    syncMode: "always",
    serverSettings: healthCheckSettings,
    syncFromServerSettings,
    resetTransient,
  });

  const updateLocalSettings = useCallback(
    (updates: Partial<HealthCheckSettings>) => {
      updateLocalSettingsInternal(updates);
    },
    [updateLocalSettingsInternal]
  );

  const handleSave = useCallback(async () => {
    if (!localSettings) {
      return;
    }

    try {
      setSaving(true);
      const updated = await updateHealthCheckSettings(localSettings);
      markSaved(updated);
      onHealthCheckSettingsChange(updated);
      toast.success("健康检测设置保存成功");
    } catch (error) {
      toast.error(`保存健康检测设置失败: ${toErrorMessage(error)}`);
    } finally {
      setSaving(false);
    }
  }, [localSettings, markSaved, onHealthCheckSettingsChange, setSaving]);

  const handleReset = useCallback(() => {
    syncFromServerSettings(healthCheckSettings);
  }, [healthCheckSettings, syncFromServerSettings]);

  return {
    saving,
    localSettings,
    hasChanges,
    updateLocalSettings,
    handleSave,
    handleReset,
  };
}

