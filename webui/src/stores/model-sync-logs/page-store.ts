import { createStore } from "zustand/vanilla";
import type { AddedModel, ModelSyncLog, ModelSyncStats, Provider } from "@/lib/api";
import { readModelSyncLogsPagePreferences, writeModelSyncLogsPagePreferences } from "@/stores/model-sync-logs/persist";
import type { ModelSyncTab, ModelSyncLogsPagePreferences } from "@/stores/model-sync-logs/types";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

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
  logs: ModelSyncLog[];
  recentModels: AddedModel[];
  recentErrors: ModelSyncLog[];
  syncTime: string;
  stats: ModelSyncStats | null;
  providersById: Record<number, Provider | undefined>;

  loading: boolean;
  recentLoading: boolean;
  errorsLoading: boolean;
  syncing: boolean;
  statsLoading: boolean;

  selectedLogs: Set<number>;
  selectedErrorProviders: Set<number>;
  page: number;
  totalPages: number;
  showUnchanged: boolean;
  togglingProviderIds: Set<number>;

  detailLog: ModelSyncLog | null;
  clearDialogOpen: boolean;
  clearingErrors: boolean;

  setActiveTab: (tab: ModelSyncTab) => void;
  setLogs: (logs: ModelSyncLog[]) => void;
  setRecentModels: (models: AddedModel[]) => void;
  setRecentErrors: (errors: ModelSyncLog[]) => void;
  setSyncTime: (syncTime: string) => void;
  setStats: (stats: ModelSyncStats | null) => void;
  setProvidersById: (next: Updater<Record<number, Provider | undefined>>) => void;

  setLoading: (loading: boolean) => void;
  setRecentLoading: (loading: boolean) => void;
  setErrorsLoading: (loading: boolean) => void;
  setSyncing: (syncing: boolean) => void;
  setStatsLoading: (loading: boolean) => void;

  setSelectedLogs: (next: Updater<Set<number>>) => void;
  setSelectedErrorProviders: (next: Updater<Set<number>>) => void;
  setPage: (page: number) => void;
  setTotalPages: (pages: number) => void;
  setShowUnchanged: (show: boolean) => void;
  setTogglingProviderIds: (next: Updater<Set<number>>) => void;

  setDetailLog: (log: ModelSyncLog | null) => void;
  setClearDialogOpen: (open: boolean) => void;
  setClearingErrors: (clearing: boolean) => void;

  resetTransient: () => void;
};

export const modelSyncLogsPageStore = createStore<ModelSyncLogsPageState>()((set, get) => ({
  activeTab: preferences.activeTab,
  logs: [],
  recentModels: [],
  recentErrors: [],
  syncTime: "",
  stats: null,
  providersById: {},

  loading: true,
  recentLoading: false,
  errorsLoading: false,
  syncing: false,
  statsLoading: true,

  selectedLogs: new Set<number>(),
  selectedErrorProviders: new Set<number>(),
  page: 1,
  totalPages: 1,
  showUnchanged: preferences.showUnchanged,
  togglingProviderIds: new Set<number>(),

  detailLog: null,
  clearDialogOpen: false,
  clearingErrors: false,

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
  setLogs: (logs: ModelSyncLog[]) => set({ logs }),
  setRecentModels: (models: AddedModel[]) => set({ recentModels: models }),
  setRecentErrors: (errors: ModelSyncLog[]) => set({ recentErrors: errors }),
  setSyncTime: (syncTime: string) => set({ syncTime }),
  setStats: (stats: ModelSyncStats | null) => set({ stats }),
  setProvidersById: (next: Updater<Record<number, Provider | undefined>>) =>
    set((state) => ({ providersById: resolveUpdater(next, state.providersById) })),

  setLoading: (loading: boolean) => set({ loading }),
  setRecentLoading: (loading: boolean) => set({ recentLoading: loading }),
  setErrorsLoading: (loading: boolean) => set({ errorsLoading: loading }),
  setSyncing: (syncing: boolean) => set({ syncing }),
  setStatsLoading: (loading: boolean) => set({ statsLoading: loading }),

  setSelectedLogs: (next: Updater<Set<number>>) => set((state) => ({ selectedLogs: resolveUpdater(next, state.selectedLogs) })),
  setSelectedErrorProviders: (next: Updater<Set<number>>) =>
    set((state) => ({ selectedErrorProviders: resolveUpdater(next, state.selectedErrorProviders) })),
  setPage: (page: number) => {
    const current = get();
    if (current.page === page) return;
    set({ page, selectedLogs: new Set() });
  },
  setTotalPages: (pages: number) => set({ totalPages: pages }),
  setShowUnchanged: (show: boolean) => {
    const current = get();
    if (current.showUnchanged === show) return;
    persistPreferences(get, { showUnchanged: show });
    set({ showUnchanged: show, page: 1, selectedLogs: new Set() });
  },
  setTogglingProviderIds: (next: Updater<Set<number>>) =>
    set((state) => ({ togglingProviderIds: resolveUpdater(next, state.togglingProviderIds) })),

  setDetailLog: (log: ModelSyncLog | null) => set({ detailLog: log }),
  setClearDialogOpen: (open: boolean) => set({ clearDialogOpen: open }),
  setClearingErrors: (clearing: boolean) => set({ clearingErrors: clearing }),

  resetTransient: () =>
    set({
      logs: [],
      recentModels: [],
      recentErrors: [],
      syncTime: "",
      stats: null,
      providersById: {},
      loading: true,
      recentLoading: false,
      errorsLoading: false,
      syncing: false,
      statsLoading: true,
      selectedLogs: new Set(),
      selectedErrorProviders: new Set(),
      page: 1,
      totalPages: 1,
      togglingProviderIds: new Set(),
      detailLog: null,
      clearDialogOpen: false,
      clearingErrors: false,
    }),
}));
