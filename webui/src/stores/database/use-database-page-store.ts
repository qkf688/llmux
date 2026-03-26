import { useStore } from "zustand";
import { databasePageStore, type DatabasePageState } from "@/stores/database/page-store";

export function useDatabasePageStore<T>(selector: (state: DatabasePageState) => T): T {
  return useStore(databasePageStore, selector);
}

