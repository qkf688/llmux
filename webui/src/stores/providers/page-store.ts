import { createStore } from "zustand/vanilla";
import type { Provider, ProviderTemplate } from "@/lib/api";
import { readProvidersPagePreferences, writeProvidersPagePreferences } from "@/stores/providers/persist";
import { DEFAULT_BATCH_TEST_PROGRESS, type BatchTestProgress, type ModelTestResult } from "@/stores/providers/types";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

export type ProvidersPageState = {
  loading: boolean;
  providers: Provider[];
  providerTemplates: ProviderTemplate[];
  nameFilter: string;
  debouncedNameFilter: string;
  typeFilter: string;
  availableTypes: string[];

  clearingAssociation: boolean;
  modelsLoading: boolean;
  addingModels: boolean;
  syncingModels: boolean;
  syncingAll: boolean;
  autoAssociateOnAddEnabled: boolean;
  autoCleanOnDeleteEnabled: boolean;

  showApiKey: boolean;

  providerDialogOpen: boolean;
  editingProvider: Provider | null;
  deleteId: number | null;
  clearAssociationId: number | null;

  modelsOpen: boolean;
  modelsOpenId: number | null;

  allModelsOpen: boolean;
  allModelsProvider: Provider | null;

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

  setLoading: (loading: boolean) => void;
  setProviders: (providers: Updater<Provider[]>) => void;
  setProviderTemplates: (templates: Updater<ProviderTemplate[]>) => void;
  setNameFilter: (value: string) => void;
  setDebouncedNameFilter: (value: string) => void;
  flushNameFilter: () => void;
  setTypeFilter: (value: string) => void;
  setAvailableTypes: (types: string[]) => void;

  setClearingAssociation: (clearing: boolean) => void;
  setModelsLoading: (loading: boolean) => void;
  setAddingModels: (adding: boolean) => void;
  setSyncingModels: (syncing: boolean) => void;
  setSyncingAll: (syncing: boolean) => void;
  setAutoAssociateOnAddEnabled: (enabled: boolean) => void;
  setAutoCleanOnDeleteEnabled: (enabled: boolean) => void;

  setShowApiKey: (show: boolean) => void;
  toggleShowApiKey: () => void;

  setEditingProvider: (provider: Provider | null) => void;
  setDeleteId: (id: number | null) => void;
  setClearAssociationId: (id: number | null) => void;
  setModelsOpenId: (id: number | null) => void;

  setProviderDialogOpen: (open: boolean) => void;
  openCreateProvider: () => void;
  openEditProvider: (provider: Provider) => void;

  openDeleteProvider: (id: number) => void;
  clearDeleteProvider: () => void;

  openClearAssociation: (id: number) => void;
  clearClearAssociation: () => void;

  openProviderModels: (providerId: number) => void;
  setModelsOpen: (open: boolean) => void;

  openAllModels: (provider: Provider) => void;
  setAllModelsOpen: (open: boolean) => void;
  setAllModelsProvider: (provider: Provider | null) => void;

  setSelectedUpstreamModels: (models: Updater<string[]>) => void;
  setSelectedAllModels: (models: Updater<string[]>) => void;
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
  loading: true,
  providers: [],
  providerTemplates: [],
  nameFilter: preferences.nameFilter,
  debouncedNameFilter: preferences.nameFilter,
  typeFilter: preferences.typeFilter,
  availableTypes: [],

  clearingAssociation: false,
  modelsLoading: false,
  addingModels: false,
  syncingModels: false,
  syncingAll: false,
  autoAssociateOnAddEnabled: false,
  autoCleanOnDeleteEnabled: false,

  showApiKey: false,

  providerDialogOpen: false,
  editingProvider: null,
  deleteId: null,
  clearAssociationId: null,

  modelsOpen: false,
  modelsOpenId: null,

  allModelsOpen: false,
  allModelsProvider: null,

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

  setLoading: (loading: boolean) => set({ loading }),
  setProviders: (providers: Updater<Provider[]>) => set((state) => ({ providers: resolveUpdater(providers, state.providers) })),
  setProviderTemplates: (templates: Updater<ProviderTemplate[]>) =>
    set((state) => ({ providerTemplates: resolveUpdater(templates, state.providerTemplates) })),
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
  setAvailableTypes: (types: string[]) => set({ availableTypes: types }),

  setClearingAssociation: (clearing: boolean) => set({ clearingAssociation: clearing }),
  setModelsLoading: (loading: boolean) => set({ modelsLoading: loading }),
  setAddingModels: (adding: boolean) => set({ addingModels: adding }),
  setSyncingModels: (syncing: boolean) => set({ syncingModels: syncing }),
  setSyncingAll: (syncing: boolean) => set({ syncingAll: syncing }),
  setAutoAssociateOnAddEnabled: (enabled: boolean) => set({ autoAssociateOnAddEnabled: enabled }),
  setAutoCleanOnDeleteEnabled: (enabled: boolean) => set({ autoCleanOnDeleteEnabled: enabled }),

  setShowApiKey: (show: boolean) => set({ showApiKey: show }),
  toggleShowApiKey: () => {
    const current = get();
    const next = !current.showApiKey;
    set({ showApiKey: next });
  },

  setEditingProvider: (provider: Provider | null) => set({ editingProvider: provider }),
  setDeleteId: (id: number | null) => set({ deleteId: id }),
  setClearAssociationId: (id: number | null) => set({ clearAssociationId: id }),
  setModelsOpenId: (id: number | null) => set({ modelsOpenId: id }),

  setProviderDialogOpen: (open: boolean) =>
    set(open ? { providerDialogOpen: true } : { providerDialogOpen: false, editingProvider: null, showApiKey: false }),
  openCreateProvider: () => set({ providerDialogOpen: true, editingProvider: null, showApiKey: false }),
  openEditProvider: (provider: Provider) => set({ providerDialogOpen: true, editingProvider: provider, showApiKey: false }),

  openDeleteProvider: (id: number) => set({ deleteId: id }),
  clearDeleteProvider: () => set({ deleteId: null }),

  openClearAssociation: (id: number) => set({ clearAssociationId: id }),
  clearClearAssociation: () => set({ clearAssociationId: null }),

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
    set({ allModelsOpen: true, allModelsProvider: provider, selectedAllModels: [], allModelsTestResults: {} }),
  setAllModelsOpen: (open: boolean) =>
    set(open ? { allModelsOpen: true } : { allModelsOpen: false, allModelsProvider: null, selectedAllModels: [], allModelsTestResults: {} }),
  setAllModelsProvider: (provider: Provider | null) => set({ allModelsProvider: provider }),

  setSelectedUpstreamModels: (models: Updater<string[]>) =>
    set((state) => ({ selectedUpstreamModels: resolveUpdater(models, state.selectedUpstreamModels) })),
  setSelectedAllModels: (models: Updater<string[]>) =>
    set((state) => ({ selectedAllModels: resolveUpdater(models, state.selectedAllModels) })),
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
      loading: true,
      providers: [],
      providerTemplates: [],
      availableTypes: [],
      providerDialogOpen: false,
      editingProvider: null,
      deleteId: null,
      clearAssociationId: null,
      modelsOpen: false,
      modelsOpenId: null,
      allModelsOpen: false,
      allModelsProvider: null,
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
      showApiKey: false,
      clearingAssociation: false,
      modelsLoading: false,
      addingModels: false,
      syncingModels: false,
      syncingAll: false,
      autoAssociateOnAddEnabled: false,
      autoCleanOnDeleteEnabled: false,
    }),
}));
