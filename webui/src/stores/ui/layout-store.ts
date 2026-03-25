import { createStore } from "zustand/vanilla";
import { readStorageBoolean, writeStorageBoolean } from "@/stores/core/storage";

const SIDEBAR_OPEN_KEY = "ui.sidebarOpen";

export type LayoutState = {
  sidebarOpen: boolean;
  setSidebarOpen: (open: boolean) => void;
  toggleSidebarOpen: () => void;
};

const initialSidebarOpen = readStorageBoolean(SIDEBAR_OPEN_KEY) ?? false;

export const layoutStore = createStore<LayoutState>()((set, get) => ({
  sidebarOpen: initialSidebarOpen,
  setSidebarOpen: (open: boolean) => {
    writeStorageBoolean(SIDEBAR_OPEN_KEY, open);
    set({ sidebarOpen: open });
  },
  toggleSidebarOpen: () => {
    const next = !get().sidebarOpen;
    writeStorageBoolean(SIDEBAR_OPEN_KEY, next);
    set({ sidebarOpen: next });
  },
}));

