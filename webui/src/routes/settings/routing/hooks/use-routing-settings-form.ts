import { useCallback, useEffect } from "react";
import { toast } from "sonner";
import { updateSettings, type Settings } from "@/lib/api";
import type { RoutingSettingsProps } from "../types";
import { sanitizeRoutingSettings } from "../utils/sanitize-routing-settings";
import { routingSettingsEditorStore, useSettingsStore } from "@/stores/settings";

export function useRoutingSettingsForm({ settings, onSettingsChange }: RoutingSettingsProps) {
  const {
    saving,
    localSettings,
    hasChanges,
    setSaving,
    syncFromServerSettings,
    updateLocalSettings: updateLocalSettingsInternal,
    markSaved,
    resetTransient,
  } = useSettingsStore(routingSettingsEditorStore, (state) => state);

  useEffect(() => {
    if (!hasChanges) {
      syncFromServerSettings(settings);
    }
  }, [hasChanges, settings, syncFromServerSettings]);

  useEffect(() => () => {
    resetTransient();
  }, [resetTransient]);

  const updateLocalSettings = useCallback(
    (updates: Partial<Settings>) => {
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
      const cleanedSettings = sanitizeRoutingSettings(localSettings);
      const updated = await updateSettings(cleanedSettings);
      markSaved(updated);
      onSettingsChange(updated);
      toast.success("通用设置保存成功");
    } catch (error) {
      toast.error("保存设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  }, [localSettings, markSaved, onSettingsChange, setSaving]);

  const handleReset = useCallback(() => {
    syncFromServerSettings(settings);
  }, [settings, syncFromServerSettings]);

  return {
    saving,
    localSettings,
    hasChanges,
    updateLocalSettings,
    handleSave,
    handleReset,
  };
}
