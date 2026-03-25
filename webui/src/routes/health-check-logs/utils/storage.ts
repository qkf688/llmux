import { healthCheckBatchStore } from "@/stores/health-check-logs";

export interface StoredHealthCheckBatchState {
  batchId: string | null;
  completed: boolean;
}

export function readStoredHealthCheckBatchState(): StoredHealthCheckBatchState {
  const state = healthCheckBatchStore.getState();
  return { batchId: state.batchId, completed: state.completed };
}

export function writeStoredHealthCheckBatchState(batchId: string, completed: boolean): void {
  healthCheckBatchStore.getState().setBatchState(batchId, completed);
}

export function markStoredHealthCheckBatchCompleted(): void {
  healthCheckBatchStore.getState().markCompleted();
}

export function clearStoredHealthCheckBatchState(): void {
  healthCheckBatchStore.getState().clearBatchState();
}
