import type { HealthCheckBatchState } from "@/stores/health-check-logs/batch-store";

export const selectHealthCheckBatchId = (state: HealthCheckBatchState) => state.batchId;
export const selectHealthCheckCompleted = (state: HealthCheckBatchState) => state.completed;
export const selectSetHealthCheckBatchState = (state: HealthCheckBatchState) => state.setBatchState;
export const selectMarkHealthCheckCompleted = (state: HealthCheckBatchState) => state.markCompleted;
export const selectClearHealthCheckBatchState = (state: HealthCheckBatchState) => state.clearBatchState;

