import { useStore } from "zustand";
import type { StoreApi } from "zustand/vanilla";

export function useSettingsStore<TState, TSelected>(
  store: StoreApi<TState>,
  selector: (state: TState) => TSelected
): TSelected {
  return useStore(store, selector);
}

