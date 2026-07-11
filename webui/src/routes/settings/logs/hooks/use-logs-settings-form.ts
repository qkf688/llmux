import { updateSettings } from "@/lib/api";
import { logsSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import { createSettingsFormHook } from "../../hooks/create-settings-form-hook";
import type { LogsSettingsProps } from "../types";

const useLogsSettingsFormInternal = createSettingsFormHook({
  useStore: () => useSettingsStore(logsSettingsEditorStore, (state) => state),
  syncMode: "always",
  successMessage: "日志设置保存成功",
  save: updateSettings,
});

export function useLogsSettingsForm(props: LogsSettingsProps) {
  return useLogsSettingsFormInternal(props);
}