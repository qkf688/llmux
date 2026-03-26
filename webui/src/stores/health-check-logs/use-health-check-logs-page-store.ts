import { useStore } from "zustand";
import { healthCheckLogsPageStore, type HealthCheckLogsPageState } from "@/stores/health-check-logs/page-store";

export function useHealthCheckLogsPageStore<T>(selector: (state: HealthCheckLogsPageState) => T): T {
  return useStore(healthCheckLogsPageStore, selector);
}

