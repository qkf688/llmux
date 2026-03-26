import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import {
  DEFAULT_HEALTH_CHECK_LOGS_FILTERS,
  HEALTH_CHECK_PAGE_SIZE_OPTIONS,
  type HealthCheckLogsFilters,
  type HealthCheckLogsPagePreferences,
} from "@/stores/health-check-logs/types";

const HEALTH_CHECK_LOGS_PREFERENCES_KEY = "healthCheckLogs.pagePreferences";

function readStoredFilters(value: unknown): HealthCheckLogsFilters {
  const stored = value as Partial<HealthCheckLogsFilters> | null | undefined;
  return {
    modelName: typeof stored?.modelName === "string" ? stored.modelName : DEFAULT_HEALTH_CHECK_LOGS_FILTERS.modelName,
    providerName:
      typeof stored?.providerName === "string" ? stored.providerName : DEFAULT_HEALTH_CHECK_LOGS_FILTERS.providerName,
    status: typeof stored?.status === "string" ? stored.status : DEFAULT_HEALTH_CHECK_LOGS_FILTERS.status,
  };
}

function readStoredPageSize(value: unknown): HealthCheckLogsPagePreferences["pageSize"] {
  const stored = typeof value === "number" ? value : null;
  return HEALTH_CHECK_PAGE_SIZE_OPTIONS.includes(stored as (typeof HEALTH_CHECK_PAGE_SIZE_OPTIONS)[number])
    ? (stored as (typeof HEALTH_CHECK_PAGE_SIZE_OPTIONS)[number])
    : 20;
}

export function readHealthCheckLogsPagePreferences(): HealthCheckLogsPagePreferences {
  const stored = readStorageJSON<Partial<HealthCheckLogsPagePreferences>>(HEALTH_CHECK_LOGS_PREFERENCES_KEY);

  return {
    filters: readStoredFilters(stored?.filters),
    pageSize: readStoredPageSize(stored?.pageSize),
  };
}

export function writeHealthCheckLogsPagePreferences(preferences: HealthCheckLogsPagePreferences): void {
  writeStorageJSON(HEALTH_CHECK_LOGS_PREFERENCES_KEY, preferences);
}

