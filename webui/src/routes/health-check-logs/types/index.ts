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

export const HEALTH_CHECK_STORAGE_KEYS = {
  batchId: "healthCheckBatchId",
  completed: "healthCheckCompleted",
} as const;
