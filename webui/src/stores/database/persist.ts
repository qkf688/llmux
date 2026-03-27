import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import {
  ALL_EXPORT_TYPES,
  DEFAULT_DATABASE_PAGE_PREFERENCES,
  type DatabasePagePreferences,
  type ImportMode,
} from "@/stores/database/types";
import type { ExportType } from "@/lib/api";

const DATABASE_PREFERENCES_KEY = "database.pagePreferences";

function readStoredExportTypes(value: unknown): ExportType[] {
  if (!Array.isArray(value)) {
    return [...DEFAULT_DATABASE_PAGE_PREFERENCES.exportTypes];
  }

  const filtered = value.filter((item): item is ExportType => typeof item === "string" && ALL_EXPORT_TYPES.includes(item as ExportType));
  return filtered.length > 0 ? filtered : [...DEFAULT_DATABASE_PAGE_PREFERENCES.exportTypes];
}

function readStoredImportMode(value: unknown): ImportMode {
  return value === "merge" || value === "replace" ? value : DEFAULT_DATABASE_PAGE_PREFERENCES.importMode;
}

export function readDatabasePagePreferences(): DatabasePagePreferences {
  const stored = readStorageJSON<Partial<DatabasePagePreferences>>(DATABASE_PREFERENCES_KEY);

  return {
    exportTypes: readStoredExportTypes(stored?.exportTypes),
    importMode: readStoredImportMode(stored?.importMode),
    importTypes: readStoredExportTypes(stored?.importTypes),
  };
}

export function writeDatabasePagePreferences(preferences: DatabasePagePreferences): void {
  writeStorageJSON(DATABASE_PREFERENCES_KEY, preferences);
}

