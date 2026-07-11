import { createStore } from "zustand/vanilla";
import type { ModelSyncLog } from "@/lib/api";
import { type Updater, resolveUpdater } from "@/stores/core/updater";
import { readModelSyncLogsPagePreferences, writeModelSyncLogsPagePreferences } from "@/stores/model-sync-logs/persist";
import type { ModelSyncTab, ModelSyncLogsPagePreferences } from "@/stores/model-sync-logs/types";

type PreferencesState = {
  activeTab: ModelSyncTab;
  showUnchanged: boolean;
};

function persistPreferences(get: () => PreferencesState, next: Partial<ModelSyncLogsPagePreferences>): void {
  const current = get();
  writeModelSyncLogsPagePreferences({
    activeTab: next.activeTab ?? current.activeTab,
    showUnchanged: next.showUnchanged ?? current.showUnchanged,
  });
}

const preferences = readModelSyncLogsPagePreferences();

export type ModelSyncLogsPageState = {
  activeTab: ModelSyncTab;
  showUnchanged: boolean;

  selectedLogs: Set<number>;
  selectedErrorProviders: Set<number>;
  page: number;
  togglingProviderIds: Set<number>;

  detailLog: ModelSyncLog | null;
  clearDialogOpen: boolean;

  setActiveTab: (tab: ModelSyncTab) => void;
  setShowUnchanged: (show: boolean) => void;

  setSelectedLogs: (next: Updater<Set<number>>) => void;
  setSelectedErrorProviders: (next: Updater<Set<number>>) => void;
  setPage: (page: number) => void;
  setTogglingProviderIds: (next: Updater<Set<number>>) => void;

  setDetailLog: (log: ModelSyncLog | null) => void;
  setClearDialogOpen: (open: boolean) => void;

  resetTransient: () => void;
};

export const modelSyncLogsPageStore = createStore<ModelSyncLogsPageState>()((set, get) => ({
  activeTab: preferences.activeTab,
  showUnchanged: preferences.showUnchanged,

  selectedLogs: new Set<number>(),
  selectedErrorProviders: new Set<number>(),
  page: 1,
  togglingProviderIds: new Set<number>(),

  detailLog: null,
  clearDialogOpen: false,

  setActiveTab: (tab: ModelSyncTab) => {
    const current = get();
    if (current.activeTab === tab) return;

    persistPreferences(get, { activeTab: tab });
    set({
      activeTab: tab,
      selectedLogs: new Set(),
      selectedErrorProviders: tab === "errors" ? current.selectedErrorProviders : new Set(),
    });
  },
  setShowUnchanged: (show: boolean) => {
    const current = get();
    if (current.showUnchanged === show) return;
    persistPreferences(get, { showUnchanged: show });
    set({ showUnchanged: show, page: 1, selectedLogs: new Set() });
  },

  setSelectedLogs: (next: Updater<Set<number>>) => set((state) => ({ selectedLogs: resolveUpdater(next, state.selectedLogs) })),
  setSelectedErrorProviders: (next: Updater<Set<number>>) =>
    set((state) => ({ selectedErrorProviders: resolveUpdater(next, state.selectedErrorProviders) })),
  setPage: (page: number) => {
    const current = get();
    if (current.page === page) return;
    set({ page, selectedLogs: new Set() });
  },
  setTogglingProviderIds: (next: Updater<Set<number>>) =>
    set((state) => ({ togglingProviderIds: resolveUpdater(next, state.togglingProviderIds) })),

  setDetailLog: (log: ModelSyncLog | null) => set({ detailLog: log }),
  setClearDialogOpen: (open: boolean) => set({ clearDialogOpen: open }),

  resetTransient: () =>
    set({
      selectedLogs: new Set(),
      selectedErrorProviders: new Set(),
      page: 1,
      togglingProviderIds: new Set(),
      detailLog: null,
      clearDialogOpen: false,
    }),
}));
