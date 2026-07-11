import { updateSettings } from "@/lib/api";
import { balancerSettingsEditorStore, useSettingsStore } from "@/stores/settings";
import { createSettingsFormHook } from "../../hooks/create-settings-form-hook";
import type { BalancerSettingsProps } from "../types";

const useBalancerSettingsFormInternal = createSettingsFormHook({
  useStore: () => useSettingsStore(balancerSettingsEditorStore, (state) => state),
  syncMode: "always",
  successMessage: "负载均衡设置保存成功",
  save: updateSettings,
});

export function useBalancerSettingsForm(props: BalancerSettingsProps) {
  return useBalancerSettingsFormInternal(props);
}