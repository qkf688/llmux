import { useCallback } from "react";
import { toast } from "sonner";
import { updateSettings, type Settings } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import { logsSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import type { LogsSettingsProps } from "../types";
import { useSettingsEditorSync } from "../../hooks/use-settings-editor-sync";

export function useLogsSettingsForm({ settings, onSettingsChange }: LogsSettingsProps) {
  const {
    saving,
    localSettings,
    hasChanges,
    setSaving,
    syncFromServerSettings,
    updateLocalSettings: updateLocalSettingsInternal,
    markSaved,
    resetTransient,
  } = useSettingsStore(logsSettingsEditorStore, (state) => state);

  useSettingsEditorSync({
    syncMode: "always",
    serverSettings: settings,
    syncFromServerSettings,
    resetTransient,
  });

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
      const updated = await updateSettings(localSettings);
      markSaved(updated);
      onSettingsChange(updated);
      toast.success("日志设置保存成功");
    } catch (error) {
      toast.error(`保存设置失败: ${toErrorMessage(error)}`);
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
