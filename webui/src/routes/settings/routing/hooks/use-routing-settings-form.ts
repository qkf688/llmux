import { updateSettings } from "@/lib/api";
import { routingSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import { createSettingsFormHook } from "../../hooks/create-settings-form-hook";
import type { RoutingSettingsProps } from "../types";
import { sanitizeRoutingSettings } from "../utils/sanitize-routing-settings";

const useRoutingSettingsFormInternal = createSettingsFormHook({
  useStore: () => useSettingsStore(routingSettingsEditorStore, (state) => state),
  syncMode: "when_clean",
  successMessage: "通用设置保存成功",
  save: updateSettings,
  sanitize: sanitizeRoutingSettings,
});

export function useRoutingSettingsForm(props: RoutingSettingsProps) {
  return useRoutingSettingsFormInternal(props);
}