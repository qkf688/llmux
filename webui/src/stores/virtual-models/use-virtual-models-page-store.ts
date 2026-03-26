import { useStore } from "zustand";
import { virtualModelsPageStore, type VirtualModelsPageState } from "@/stores/virtual-models/page-store";

export function useVirtualModelsPageStore<T>(selector: (state: VirtualModelsPageState) => T): T {
  return useStore(virtualModelsPageStore, selector);
}

