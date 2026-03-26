import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import { DEFAULT_LOGS_FILTERS, type LogsFilters, type LogsPagePreferences } from "@/stores/logs/types";

const LOGS_PREFERENCES_KEY = "logs.pagePreferences";

function normalizeFilters(filters: Partial<LogsFilters> | undefined): LogsFilters {
  return {
    ...DEFAULT_LOGS_FILTERS,
    ...filters,
  };
}

export function readLogsPagePreferences(): LogsPagePreferences {
  const stored = readStorageJSON<Partial<LogsPagePreferences>>(LOGS_PREFERENCES_KEY);

  const filters = normalizeFilters(stored?.filters);
  const pageSize = typeof stored?.pageSize === "number" && stored.pageSize > 0 ? stored.pageSize : 20;

  return { filters, pageSize };
}

export function writeLogsPagePreferences(preferences: LogsPagePreferences): void {
  writeStorageJSON(LOGS_PREFERENCES_KEY, preferences);
}

