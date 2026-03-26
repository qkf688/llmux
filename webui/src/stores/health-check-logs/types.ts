export interface HealthCheckLogsFilters {
  modelName: string;
  providerName: string;
  status: string;
}

export const DEFAULT_HEALTH_CHECK_LOGS_FILTERS: HealthCheckLogsFilters = {
  modelName: "all",
  providerName: "all",
  status: "all",
};

export const HEALTH_CHECK_PAGE_SIZE_OPTIONS = [10, 20, 50] as const;

export type HealthCheckLogsPagePreferences = {
  filters: HealthCheckLogsFilters;
  pageSize: (typeof HEALTH_CHECK_PAGE_SIZE_OPTIONS)[number];
};

