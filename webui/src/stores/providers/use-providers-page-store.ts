import { useStore } from "zustand";
import { providersPageStore, type ProvidersPageState } from "@/stores/providers/page-store";

export function useProvidersPageStore<T>(selector: (state: ProvidersPageState) => T): T {
  return useStore(providersPageStore, selector);
}

