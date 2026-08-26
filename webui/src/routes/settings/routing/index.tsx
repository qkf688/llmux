import { CapabilityMatchingCard } from "./components/sections/capability-matching-card";
import { FormatConversionCard } from "./components/sections/format-conversion-card";
import { ModelAssociationAutomationCard } from "./components/sections/model-association-automation-card";
import { ModelSyncCard } from "./components/sections/model-sync-card";
import { ParameterMappingCard } from "./components/sections/parameter-mapping-card";
import { RequestParamsCard } from "./components/sections/request-params-card";
import { TemplateFuzzyMatchCard } from "./components/sections/template-fuzzy-match-card";
import { RoutingSettingsHeader } from "./components/shared/routing-settings-header";
import { useRoutingSettingsForm } from "./hooks/use-routing-settings-form";
import type { RoutingSettingsProps } from "./types";

export function RoutingSettings(props: RoutingSettingsProps) {
  const form = useRoutingSettingsForm(props);

  return (
    <div className="space-y-4 md:space-y-6">
      <RoutingSettingsHeader
        saving={form.saving}
        hasChanges={form.hasChanges}
        onReset={form.handleReset}
        onSave={() => {
          void form.handleSave();
        }}
      />

      <CapabilityMatchingCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <ModelSyncCard localSettings={form.localSettings} updateLocalSettings={form.updateLocalSettings} />
      <TemplateFuzzyMatchCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <ModelAssociationAutomationCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <FormatConversionCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
      <RequestParamsCard localSettings={form.localSettings} updateLocalSettings={form.updateLocalSettings} />
      <ParameterMappingCard
        localSettings={form.localSettings}
        updateLocalSettings={form.updateLocalSettings}
      />
    </div>
  );
}
