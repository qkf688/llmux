import { useStore } from "zustand";
import { layoutStore, type LayoutState } from "@/stores/ui/layout-store";

export function useLayoutStore<T>(selector: (state: LayoutState) => T): T {
  return useStore(layoutStore, selector);
}

