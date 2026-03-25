import type { LayoutState } from "@/stores/ui/layout-store";

export const selectSidebarOpen = (state: LayoutState) => state.sidebarOpen;
export const selectToggleSidebarOpen = (state: LayoutState) => state.toggleSidebarOpen;
export const selectSetSidebarOpen = (state: LayoutState) => state.setSidebarOpen;

