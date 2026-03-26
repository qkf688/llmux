import type { HealthCheckSettings, Settings } from "@/lib/api";
import { createSettingsEditorStore } from "@/stores/settings/editor-store";

export const logsSettingsEditorStore = createSettingsEditorStore<Settings>();
export const balancerSettingsEditorStore = createSettingsEditorStore<Settings>();
export const routingSettingsEditorStore = createSettingsEditorStore<Settings>();
export const healthCheckSettingsEditorStore = createSettingsEditorStore<HealthCheckSettings>();

