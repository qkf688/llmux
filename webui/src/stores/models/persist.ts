import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import { DEFAULT_MODELS_PAGE_PREFERENCES, type ModelsPagePreferences } from "@/stores/models/types";

const MODELS_PREFERENCES_KEY = "models.pagePreferences";

export function readModelsPagePreferences(): ModelsPagePreferences {
  const stored = readStorageJSON<Partial<ModelsPagePreferences>>(MODELS_PREFERENCES_KEY);
  return {
    ...DEFAULT_MODELS_PAGE_PREFERENCES,
    ...stored,
  };
}

export function writeModelsPagePreferences(preferences: ModelsPagePreferences): void {
  writeStorageJSON(MODELS_PREFERENCES_KEY, preferences);
}

