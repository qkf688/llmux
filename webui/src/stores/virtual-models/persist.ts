import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import { DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES, type VirtualModelsPagePreferences } from "@/stores/virtual-models/types";

const VIRTUAL_MODELS_PREFERENCES_KEY = "virtualModels.pagePreferences";

function readStoredNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

export function readVirtualModelsPagePreferences(): VirtualModelsPagePreferences {
  const stored = readStorageJSON<Partial<VirtualModelsPagePreferences>>(VIRTUAL_MODELS_PREFERENCES_KEY);

  return {
    modelSearchQuery: typeof stored?.modelSearchQuery === "string" ? stored.modelSearchQuery : DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES.modelSearchQuery,
    providerSearchQuery:
      typeof stored?.providerSearchQuery === "string" ? stored.providerSearchQuery : DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES.providerSearchQuery,
    batchPriority: readStoredNumber(stored?.batchPriority, DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES.batchPriority),
    batchWeight: readStoredNumber(stored?.batchWeight, DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES.batchWeight),
    batchEnabled: typeof stored?.batchEnabled === "boolean" ? stored.batchEnabled : DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES.batchEnabled,
  };
}

export function writeVirtualModelsPagePreferences(preferences: VirtualModelsPagePreferences): void {
  writeStorageJSON(VIRTUAL_MODELS_PREFERENCES_KEY, preferences);
}

