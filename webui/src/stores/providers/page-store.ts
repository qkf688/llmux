import { createStore } from "zustand/vanilla";
import type { Provider } from "@/lib/api";
import { type Updater, resolveUpdater } from "@/stores/core/updater";
import { readProvidersPagePreferences, writeProvidersPagePreferences } from "@/stores/providers/persist";
import { DEFAULT_BATCH_TEST_PROGRESS, type AllModelsTypeFilter, type BatchTestProgress, type ModelTestResult } from "@/stores/providers/types";

export type ProvidersPageState = {
  nameFilter: string;
  debouncedNameFilter: string;
  typeFilter: string;

  modelsLoading: boolean;
  addingModels: boolean;
  syncingModels: boolean;
  syncingAll: boolean;

  providerDialogOpen: boolean;
  editingProvider: Provider | null;
  /** 编辑弹窗详情加载中：提交按钮据此禁用，防止空默认值全量覆盖子表 */
  detailLoading: boolean;

  modelsOpen: boolean;
  modelsOpenId: number | null;

  allModelsOpen: boolean;
  allModelsProvider: Provider | null;

  allModelsTypeFilter: AllModelsTypeFilter;

  selectedUpstreamModels: string[];
  selectedAllModels: string[];
  allModelsSearchQuery: string;
  customModelInput: string;

  allModelsTestResults: Record<string, ModelTestResult>;
  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;

  upstreamTestResults: Record<string, ModelTestResult>;
  upstreamBatchTesting: boolean;
  upstreamBatchTestProgress: BatchTestProgress;

  setNameFilter: (value: string) => void;
  setDebouncedNameFilter: (value: string) => void;
  flushNameFilter: () => void;
  setTypeFilter: (value: string) => void;

  setModelsLoading: (loading: boolean) => void;
  setAddingModels: (adding: boolean) => void;
  setSyncingModels: (syncing: boolean) => void;
  setSyncingAll: (syncing: boolean) => void;

  setEditingProvider: (provider: Provider | null) => void;
  setModelsOpenId: (id: number | null) => void;
  setDetailLoading: (loading: boolean) => void;

  setProviderDialogOpen: (open: boolean) => void;

  openProviderModels: (providerId: number) => void;
  setModelsOpen: (open: boolean) => void;

  openAllModels: (provider: Provider) => void;
  setAllModelsOpen: (open: boolean) => void;
  setAllModelsProvider: (provider: Provider | null) => void;

  setSelectedUpstreamModels: (models: Updater<string[]>) => void;
  setSelectedAllModels: (models: Updater<string[]>) => void;
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => void;
  setAllModelsSearchQuery: (query: string) => void;
  setCustomModelInput: (value: string) => void;

  setAllModelsTestResults: (results: Updater<Record<string, ModelTestResult>>) => void;
  setBatchTesting: (testing: boolean) => void;
  setBatchTestProgress: (progress: Updater<BatchTestProgress>) => void;

  setUpstreamTestResults: (results: Updater<Record<string, ModelTestResult>>) => void;
  setUpstreamBatchTesting: (testing: boolean) => void;
  setUpstreamBatchTestProgress: (progress: Updater<BatchTestProgress>) => void;

  resetTransient: () => void;
};

const preferences = readProvidersPagePreferences();

export const providersPageStore = createStore<ProvidersPageState>()((set, get) => ({
  nameFilter: preferences.nameFilter,
  debouncedNameFilter: preferences.nameFilter,
  typeFilter: preferences.typeFilter,

  modelsLoading: false,
  addingModels: false,
  syncingModels: false,
  syncingAll: false,

  providerDialogOpen: false,
  editingProvider: null,
  detailLoading: false,

  modelsOpen: false,
  modelsOpenId: null,

  allModelsOpen: false,
  allModelsProvider: null,

  allModelsTypeFilter: "all",

  selectedUpstreamModels: [],
  selectedAllModels: [],
  allModelsSearchQuery: "",
  customModelInput: "",

  allModelsTestResults: {},
  batchTesting: false,
  batchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },

  upstreamTestResults: {},
  upstreamBatchTesting: false,
  upstreamBatchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },

  setNameFilter: (value: string) => {
    set({ nameFilter: value });
    const current = get();
    writeProvidersPagePreferences({ nameFilter: value, typeFilter: current.typeFilter });
  },
  setDebouncedNameFilter: (value: string) => set({ debouncedNameFilter: value }),
  flushNameFilter: () => {
    const current = get();
    set({ debouncedNameFilter: current.nameFilter });
  },
  setTypeFilter: (value: string) => {
    set({ typeFilter: value });
    const current = get();
    writeProvidersPagePreferences({ nameFilter: current.nameFilter, typeFilter: value });
  },

  setModelsLoading: (loading: boolean) => set({ modelsLoading: loading }),
  setAddingModels: (adding: boolean) => set({ addingModels: adding }),
  setSyncingModels: (syncing: boolean) => set({ syncingModels: syncing }),
  setSyncingAll: (syncing: boolean) => set({ syncingAll: syncing }),

  setEditingProvider: (provider: Provider | null) => set({ editingProvider: provider }),
  setModelsOpenId: (id: number | null) => set({ modelsOpenId: id }),
  setDetailLoading: (loading: boolean) => set({ detailLoading: loading }),

  setProviderDialogOpen: (open: boolean) =>
    set(open ? { providerDialogOpen: true } : { providerDialogOpen: false, editingProvider: null }),

  openProviderModels: (providerId: number) =>
    set({
      modelsOpen: true,
      modelsOpenId: providerId,
      selectedUpstreamModels: [],
      upstreamTestResults: {},
      upstreamBatchTesting: false,
      upstreamBatchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },
    }),
  setModelsOpen: (open: boolean) =>
    set(
      open
        ? { modelsOpen: true }
        : {
            modelsOpen: false,
            modelsOpenId: null,
            selectedUpstreamModels: [],
            upstreamTestResults: {},
            upstreamBatchTesting: false,
            upstreamBatchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },
          },
    ),

  openAllModels: (provider: Provider) =>
    set({
      allModelsOpen: true,
      allModelsProvider: provider,
      selectedAllModels: [],
      allModelsTestResults: {},
      allModelsTypeFilter: "all",
      allModelsSearchQuery: "",
    }),
  setAllModelsOpen: (open: boolean) =>
    set(open ? { allModelsOpen: true } : { allModelsOpen: false, allModelsProvider: null, selectedAllModels: [], allModelsTestResults: {} }),
  setAllModelsProvider: (provider: Provider | null) => set({ allModelsProvider: provider }),

  setSelectedUpstreamModels: (models: Updater<string[]>) =>
    set((state) => ({ selectedUpstreamModels: resolveUpdater(models, state.selectedUpstreamModels) })),
  setSelectedAllModels: (models: Updater<string[]>) =>
    set((state) => ({ selectedAllModels: resolveUpdater(models, state.selectedAllModels) })),
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => set({ allModelsTypeFilter: value }),
  setAllModelsSearchQuery: (query: string) => set({ allModelsSearchQuery: query }),
  setCustomModelInput: (value: string) => set({ customModelInput: value }),

  setAllModelsTestResults: (results: Updater<Record<string, ModelTestResult>>) =>
    set((state) => ({ allModelsTestResults: resolveUpdater(results, state.allModelsTestResults) })),
  setBatchTesting: (testing: boolean) => set({ batchTesting: testing }),
  setBatchTestProgress: (progress: Updater<BatchTestProgress>) =>
    set((state) => ({ batchTestProgress: resolveUpdater(progress, state.batchTestProgress) })),

  setUpstreamTestResults: (results: Updater<Record<string, ModelTestResult>>) =>
    set((state) => ({ upstreamTestResults: resolveUpdater(results, state.upstreamTestResults) })),
  setUpstreamBatchTesting: (testing: boolean) => set({ upstreamBatchTesting: testing }),
  setUpstreamBatchTestProgress: (progress: Updater<BatchTestProgress>) =>
    set((state) => ({ upstreamBatchTestProgress: resolveUpdater(progress, state.upstreamBatchTestProgress) })),

  resetTransient: () =>
    set({
      providerDialogOpen: false,
      editingProvider: null,
      detailLoading: false,
      modelsOpen: false,
      modelsOpenId: null,
      allModelsOpen: false,
      allModelsProvider: null,
      allModelsTypeFilter: "all",
      selectedUpstreamModels: [],
      selectedAllModels: [],
      allModelsSearchQuery: "",
      customModelInput: "",
      allModelsTestResults: {},
      batchTesting: false,
      batchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },
      upstreamTestResults: {},
      upstreamBatchTesting: false,
      upstreamBatchTestProgress: { ...DEFAULT_BATCH_TEST_PROGRESS },
      modelsLoading: false,
      addingModels: false,
      syncingModels: false,
      syncingAll: false,
    }),
}));
