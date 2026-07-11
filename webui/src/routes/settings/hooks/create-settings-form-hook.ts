import { useCallback } from "react";
import { toast } from "sonner";
import { toErrorMessage } from "@/lib/errors";
import type { SettingsEditorState } from "@/stores/settings";
import {
  useSettingsEditorSync,
  type SettingsEditorSyncMode,
} from "./use-settings-editor-sync";

export type SettingsFormHookProps<TSettings extends object> = {
  settings: TSettings | null;
  onSettingsChange: (settings: TSettings) => void;
};

export type CreateSettingsFormHookOptions<TSettings extends object> = {
  /** 绑定具体 editor store 的订阅函数 */
  useStore: () => SettingsEditorState<TSettings>;
  syncMode: SettingsEditorSyncMode;
  successMessage: string;
  save: (local: TSettings) => Promise<TSettings>;
  sanitize?: (local: TSettings) => TSettings;
  /** 失败 toast 前缀，默认「保存设置失败」 */
  errorMessagePrefix?: string;
};

/**
 * 统一 settings 表单 hook：store 同步 local、updateLocal、save/reset、saving、try/catch、toast。
 * 差异通过 options 注入；props 命名差异由各模块薄包装适配。
 */
export function createSettingsFormHook<TSettings extends object>(
  options: CreateSettingsFormHookOptions<TSettings>
) {
  const {
    useStore,
    syncMode,
    successMessage,
    save,
    sanitize,
    errorMessagePrefix = "保存设置失败",
  } = options;

  return function useSettingsForm({
    settings,
    onSettingsChange,
  }: SettingsFormHookProps<TSettings>) {
    const {
      saving,
      localSettings,
      hasChanges,
      setSaving,
      syncFromServerSettings,
      updateLocalSettings: updateLocalSettingsInternal,
      markSaved,
      resetTransient,
    } = useStore();

    useSettingsEditorSync(
      syncMode === "always"
        ? {
            syncMode: "always",
            serverSettings: settings,
            syncFromServerSettings,
            resetTransient,
          }
        : {
            syncMode: "when_clean",
            hasChanges,
            serverSettings: settings,
            syncFromServerSettings,
            resetTransient,
          }
    );

    const updateLocalSettings = useCallback(
      (updates: Partial<TSettings>) => {
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
        const payload = sanitize ? sanitize(localSettings) : localSettings;
        const updated = await save(payload);
        markSaved(updated);
        onSettingsChange(updated);
        toast.success(successMessage);
      } catch (error) {
        toast.error(`${errorMessagePrefix}: ${toErrorMessage(error)}`);
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
  };
}