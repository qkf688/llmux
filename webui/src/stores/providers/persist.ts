import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import type { ProvidersPagePreferences } from "@/stores/providers/types";

const PROVIDERS_PREFERENCES_KEY = "providers.pagePreferences";

export function readProvidersPagePreferences(): ProvidersPagePreferences {
  const stored = readStorageJSON<Partial<ProvidersPagePreferences>>(PROVIDERS_PREFERENCES_KEY);

  return {
    nameFilter: typeof stored?.nameFilter === "string" ? stored.nameFilter : "",
    typeFilter: typeof stored?.typeFilter === "string" ? stored.typeFilter : "all",
  };
}

export function writeProvidersPagePreferences(preferences: ProvidersPagePreferences): void {
  writeStorageJSON(PROVIDERS_PREFERENCES_KEY, preferences);
}
