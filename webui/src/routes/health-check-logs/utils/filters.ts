import type { HealthCheckLogsFilters } from "../types";

export function toHealthCheckLogsApiFilters(filters: HealthCheckLogsFilters): {
  providerName?: string;
  modelName?: string;
  status?: string;
} {
  return {
    providerName: filters.providerName === "all" ? undefined : filters.providerName,
    modelName: filters.modelName === "all" ? undefined : filters.modelName,
    status: filters.status === "all" ? undefined : filters.status,
  };
}
