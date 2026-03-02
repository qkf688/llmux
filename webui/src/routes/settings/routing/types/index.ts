import type { Settings } from "@/lib/api";

export interface RoutingSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export interface RoutingSettingsSectionProps {
  localSettings: Settings | null;
  updateLocalSettings: (updates: Partial<Settings>) => void;
}
