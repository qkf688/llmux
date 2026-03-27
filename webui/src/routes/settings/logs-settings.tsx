import { LogsConfigurationCard } from "./logs/components/sections/logs-configuration-card";
import { PerformanceOptimizationCard } from "./logs/components/sections/performance-optimization-card";
import { LogsSettingsHeader } from "./logs/components/shared/logs-settings-header";
import { useLogsSettingsForm } from "./logs/hooks/use-logs-settings-form";
import type { LogsSettingsProps } from "./logs/types";

export function LogsSettings(props: LogsSettingsProps) {
  const form = useLogsSettingsForm(props);

  return (
    <div className="space-y-4 md:space-y-6">
      <LogsSettingsHeader
        saving={form.saving}
        hasChanges={form.hasChanges}
        onReset={form.handleReset}
        onSave={() => {
          void form.handleSave();
        }}
      />

      <LogsConfigurationCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <PerformanceOptimizationCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
    </div>
  );
}

