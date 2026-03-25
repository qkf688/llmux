import { createStore } from "zustand/vanilla";
import { readStorageBoolean, readStorageString, removeStorageItem, writeStorageBoolean, writeStorageString } from "@/stores/core/storage";

const STORAGE_KEYS = {
  batchId: "healthCheckBatchId",
  completed: "healthCheckCompleted",
} as const;

export type HealthCheckBatchState = {
  batchId: string | null;
  completed: boolean;
  setBatchState: (batchId: string, completed: boolean) => void;
  markCompleted: () => void;
  clearBatchState: () => void;
};

const initialBatchId = readStorageString(STORAGE_KEYS.batchId);
const initialCompleted = readStorageBoolean(STORAGE_KEYS.completed) ?? false;

export const healthCheckBatchStore = createStore<HealthCheckBatchState>()((set, get) => ({
  batchId: initialBatchId,
  completed: initialCompleted,
  setBatchState: (batchId: string, completed: boolean) => {
    writeStorageString(STORAGE_KEYS.batchId, batchId);
    writeStorageBoolean(STORAGE_KEYS.completed, completed);
    set({ batchId, completed });
  },
  markCompleted: () => {
    const current = get();
    writeStorageBoolean(STORAGE_KEYS.completed, true);
    set({ ...current, completed: true });
  },
  clearBatchState: () => {
    removeStorageItem(STORAGE_KEYS.batchId);
    removeStorageItem(STORAGE_KEYS.completed);
    set({ batchId: null, completed: false });
  },
}));

