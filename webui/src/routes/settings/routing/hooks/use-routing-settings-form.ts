import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { updateSettings, type Settings } from "@/lib/api";
import type { RoutingSettingsProps } from "../types";
import { sanitizeRoutingSettings } from "../utils/sanitize-routing-settings";

export function useRoutingSettingsForm({ settings, onSettingsChange }: RoutingSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(settings);
  const [hasChanges, setHasChanges] = useState(false);

  useEffect(() => {
    if (!hasChanges) {
      setLocalSettings(settings);
    }
  }, [settings, hasChanges]);

  const updateLocalSettings = useCallback((updates: Partial<Settings>) => {
    setLocalSettings((previous) => {
      if (!previous) {
        return previous;
      }
      return { ...previous, ...updates };
    });
    setHasChanges(true);
  }, []);

  const handleSave = useCallback(async () => {
    if (!localSettings) {
      return;
    }

    try {
      setSaving(true);
      const cleanedSettings = sanitizeRoutingSettings(localSettings);
      const updated = await updateSettings(cleanedSettings);
      setLocalSettings(updated);
      onSettingsChange(updated);
      setHasChanges(false);
      toast.success("通用设置保存成功");
    } catch (error) {
      toast.error("保存设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  }, [localSettings, onSettingsChange]);

  const handleReset = useCallback(() => {
    setLocalSettings(settings);
    setHasChanges(false);
  }, [settings]);

  return {
    saving,
    localSettings,
    hasChanges,
    updateLocalSettings,
    handleSave,
    handleReset,
  };
}
