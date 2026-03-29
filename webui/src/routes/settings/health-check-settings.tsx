import { BasicSettingsCard } from "./health-check/components/sections/basic-settings-card";
import { FailureHandlingCard } from "./health-check/components/sections/failure-handling-card";
import { InvocationCountPolicyCard } from "./health-check/components/sections/invocation-count-policy-card";
import { HealthCheckSettingsHeader } from "./health-check/components/shared/health-check-settings-header";
import { useHealthCheckSettingsForm } from "./health-check/hooks/use-health-check-settings-form";
import type { HealthCheckSettingsProps } from "./health-check/types";

export function HealthCheckSettingsTab(props: HealthCheckSettingsProps) {
  const form = useHealthCheckSettingsForm(props);

  return (
    <div className="space-y-4 md:space-y-6">
      <HealthCheckSettingsHeader
        saving={form.saving}
        hasChanges={form.hasChanges}
        onReset={form.handleReset}
        onSave={() => {
          void form.handleSave();
        }}
      />

      <BasicSettingsCard localSettings={form.localSettings} updateLocalSettings={form.updateLocalSettings} />
      <FailureHandlingCard localSettings={form.localSettings} updateLocalSettings={form.updateLocalSettings} />
      <InvocationCountPolicyCard localSettings={form.localSettings} updateLocalSettings={form.updateLocalSettings} />
    </div>
  );
}
