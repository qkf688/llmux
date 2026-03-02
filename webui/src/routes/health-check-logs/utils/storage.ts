import { HEALTH_CHECK_STORAGE_KEYS } from "../types";

export interface StoredHealthCheckBatchState {
  batchId: string | null;
  completed: boolean;
}

export function readStoredHealthCheckBatchState(): StoredHealthCheckBatchState {
  const batchId = localStorage.getItem(HEALTH_CHECK_STORAGE_KEYS.batchId);
  const completed = localStorage.getItem(HEALTH_CHECK_STORAGE_KEYS.completed) === "true";

  return { batchId, completed };
}

export function writeStoredHealthCheckBatchState(batchId: string, completed: boolean): void {
  localStorage.setItem(HEALTH_CHECK_STORAGE_KEYS.batchId, batchId);
  localStorage.setItem(HEALTH_CHECK_STORAGE_KEYS.completed, String(completed));
}

export function markStoredHealthCheckBatchCompleted(): void {
  localStorage.setItem(HEALTH_CHECK_STORAGE_KEYS.completed, "true");
}

export function clearStoredHealthCheckBatchState(): void {
  localStorage.removeItem(HEALTH_CHECK_STORAGE_KEYS.batchId);
  localStorage.removeItem(HEALTH_CHECK_STORAGE_KEYS.completed);
}
