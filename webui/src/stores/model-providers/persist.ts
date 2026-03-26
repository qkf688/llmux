import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import type { ModelProvidersPagePreferences } from "@/stores/model-providers/types";

const MODEL_PROVIDERS_PREFERENCES_KEY = "modelProviders.pagePreferences";

function getDefaultFilterPanelOpen(): boolean {
  if (typeof window !== "undefined") {
    return window.innerWidth >= 640;
  }
  return true;
}

export function readModelProvidersPagePreferences(): ModelProvidersPagePreferences {
  const stored = readStorageJSON<Partial<ModelProvidersPagePreferences>>(MODEL_PROVIDERS_PREFERENCES_KEY);

  return {
    searchKeyword: typeof stored?.searchKeyword === "string" ? stored.searchKeyword : "",
    selectedProviderType: typeof stored?.selectedProviderType === "string" ? stored.selectedProviderType : "all",
    selectedProviderFilter: typeof stored?.selectedProviderFilter === "string" ? stored.selectedProviderFilter : "all",
    selectedStatusFilter: typeof stored?.selectedStatusFilter === "string" ? stored.selectedStatusFilter : "all",
    filterPanelOpen: typeof stored?.filterPanelOpen === "boolean" ? stored.filterPanelOpen : getDefaultFilterPanelOpen(),
    operationScope: stored?.operationScope === "all" ? "all" : "current",
  };
}

export function writeModelProvidersPagePreferences(preferences: ModelProvidersPagePreferences): void {
  writeStorageJSON(MODEL_PROVIDERS_PREFERENCES_KEY, preferences);
}

