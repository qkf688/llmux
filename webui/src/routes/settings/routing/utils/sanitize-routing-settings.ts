import type { Settings } from "@/lib/api";

export function sanitizeRoutingSettings(settings: Settings): Settings {
  return {
    ...settings,
    model_sync_filter_rules: settings.model_sync_filter_rules
      ?.map((rule) => rule.trim())
      .filter(Boolean) ?? [],
  };
}
