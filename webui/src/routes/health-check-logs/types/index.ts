export type { HealthCheckLogsFilters } from "@/stores/health-check-logs/types";
export { DEFAULT_HEALTH_CHECK_LOGS_FILTERS, HEALTH_CHECK_PAGE_SIZE_OPTIONS } from "@/stores/health-check-logs/types";

export const HEALTH_CHECK_STORAGE_KEYS = {
  batchId: "healthCheckBatchId",
  completed: "healthCheckCompleted",
} as const;
