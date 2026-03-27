import type { Settings } from "@/lib/api";

export interface BalancerSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export interface BalancerSettingsSectionProps {
  localSettings: Settings | null;
  updateLocalSettings: (updates: Partial<Settings>) => void;
}

