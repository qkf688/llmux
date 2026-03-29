import type { HealthCheckSettings } from "@/lib/api";

export interface HealthCheckSettingsProps {
  healthCheckSettings: HealthCheckSettings | null;
  onHealthCheckSettingsChange: (settings: HealthCheckSettings) => void;
}

export interface HealthCheckSettingsSectionProps {
  localSettings: HealthCheckSettings | null;
  updateLocalSettings: (updates: Partial<HealthCheckSettings>) => void;
}

