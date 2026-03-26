import { useStore } from "zustand";
import { modelsPageStore, type ModelsPageState } from "@/stores/models/page-store";

export function useModelsPageStore<T>(selector: (state: ModelsPageState) => T): T {
  return useStore(modelsPageStore, selector);
}

