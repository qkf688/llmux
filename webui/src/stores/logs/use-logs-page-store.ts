import { useStore } from "zustand";
import { logsPageStore, type LogsPageState } from "@/stores/logs/page-store";

export function useLogsPageStore<T>(selector: (state: LogsPageState) => T): T {
  return useStore(logsPageStore, selector);
}

