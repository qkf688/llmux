import { updateHealthCheckSettings } from "@/lib/api";
import { healthCheckSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import { createSettingsFormHook } from "../../hooks/create-settings-form-hook";
import type { HealthCheckSettingsProps } from "../types";

const useHealthCheckSettingsFormInternal = createSettingsFormHook({
  useStore: () => useSettingsStore(healthCheckSettingsEditorStore, (state) => state),
  syncMode: "always",
  successMessage: "健康检测设置保存成功",
  save: updateHealthCheckSettings,
  errorMessagePrefix: "保存健康检测设置失败",
});

/** props 命名与通用 settings 不同，在此适配后交给工厂 hook */
export function useHealthCheckSettingsForm({
  healthCheckSettings,
  onHealthCheckSettingsChange,
}: HealthCheckSettingsProps) {
  return useHealthCheckSettingsFormInternal({
    settings: healthCheckSettings,
    onSettingsChange: onHealthCheckSettingsChange,
  });
}