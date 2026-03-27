import { AutoPriorityDecayCard } from "./balancer/components/sections/auto-priority-decay-card";
import { AutoSuccessIncreaseCard } from "./balancer/components/sections/auto-success-increase-card";
import { AutoWeightDecayCard } from "./balancer/components/sections/auto-weight-decay-card";
import { ConsecutiveFailureDisableCard } from "./balancer/components/sections/consecutive-failure-disable-card";
import { BalancerSettingsHeader } from "./balancer/components/shared/balancer-settings-header";
import { useBalancerSettingsForm } from "./balancer/hooks/use-balancer-settings-form";
import type { BalancerSettingsProps } from "./balancer/types";

export function BalancerSettings(props: BalancerSettingsProps) {
  const form = useBalancerSettingsForm(props);

  return (
    <div className="space-y-4 md:space-y-6">
      <BalancerSettingsHeader
        saving={form.saving}
        hasChanges={form.hasChanges}
        onReset={form.handleReset}
        onSave={() => {
          void form.handleSave();
        }}
      />

      <AutoWeightDecayCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <AutoPriorityDecayCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <ConsecutiveFailureDisableCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <AutoSuccessIncreaseCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
    </div>
  );
}

