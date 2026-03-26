import { useStore } from "zustand";
import { modelSyncLogsPageStore, type ModelSyncLogsPageState } from "@/stores/model-sync-logs/page-store";

export function useModelSyncLogsPageStore<T>(selector: (state: ModelSyncLogsPageState) => T): T {
  return useStore(modelSyncLogsPageStore, selector);
}

