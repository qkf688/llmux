import type { Settings } from "@/lib/api";

export interface LogsSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export interface LogsSettingsSectionProps {
  localSettings: Settings | null;
  updateLocalSettings: (updates: Partial<Settings>) => void;
}

