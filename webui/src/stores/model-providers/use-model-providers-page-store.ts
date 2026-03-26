import { useStore } from "zustand";
import { modelProvidersPageStore, type ModelProvidersPageState } from "@/stores/model-providers/page-store";

export function useModelProvidersPageStore<T>(selector: (state: ModelProvidersPageState) => T): T {
  return useStore(modelProvidersPageStore, selector);
}

