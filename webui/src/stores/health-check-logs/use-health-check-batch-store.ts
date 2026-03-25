import { useStore } from "zustand";
import { healthCheckBatchStore, type HealthCheckBatchState } from "@/stores/health-check-logs/batch-store";

export function useHealthCheckBatchStore<T>(selector: (state: HealthCheckBatchState) => T): T {
  return useStore(healthCheckBatchStore, selector);
}

