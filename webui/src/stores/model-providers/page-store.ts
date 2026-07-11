import { createStore } from "zustand/vanilla";
import type { ModelWithProvider } from "@/lib/api";
import { type Updater, resolveUpdater } from "@/stores/core/updater";
import { readModelProvidersPagePreferences, writeModelProvidersPagePreferences } from "@/stores/model-providers/persist";
import type {
  AssociationBatchTestResult,
  BatchTestProgress,
  BlacklistFilter,
  ModelProvidersPagePreferences,
  ProviderModelSelection,
  ReactTestResultState,
  TestType,
} from "@/stores/model-providers/types";

function getInitialBatchTestProgress(): BatchTestProgress {
  return { total: 0, completed: 0, success: 0, failed: 0, testing: 0 };
}

function getInitialReactTestResult(): ReactTestResultState {
  return { loading: false, messages: "", success: null, error: null };
}

const preferences = readModelProvidersPagePreferences();

export type ModelProvidersPageState = {
  open: boolean;
  editingAssociation: ModelWithProvider | null;
  deleteId: number | null;

  testDialogOpen: boolean;
  selectedTestId: number | null;
  testType: TestType;
  isSubmitting: boolean;

  modelListDialogOpen: boolean;
  modelSearchKeyword: string;
  selectedProviderModels: ProviderModelSelection[];
  collapsedProviders: Record<number, boolean>;
  selectedAssociationIds: number[];

  batchDeleteDialogOpen: boolean;
  batchDeleting: boolean;
  batchUpdatingStatus: boolean;
  batchActionSheetOpen: boolean;
  batchCapabilitiesDialogOpen: boolean;
  batchUpdatingCapabilities: boolean;

  searchKeyword: string;
  selectedProviderType: string;
  selectedProviderFilter: string;
  selectedStatusFilter: string;
  filterPanelOpen: boolean;
  operationScope: "current" | "all";

  previewDialogOpen: boolean;
  previewType: "associate" | "clean";
  executing: boolean;

  templateEditorOpen: boolean;
  templateLoading: boolean;
  templateNewItem: string;

  resettingWeights: boolean;
  resettingPriorities: boolean;
  enablingAssociations: boolean;

  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;
  associationTestResults: Record<number, AssociationBatchTestResult>;

  reactTestResult: ReactTestResultState;

  blacklistDialogOpen: boolean;
  blacklistedIds: number[];
  blacklistLoading: boolean;
  blacklistSaving: boolean;
  blacklistSearchTerm: string;
  blacklistFilter: BlacklistFilter;

  setOpen: (open: boolean) => void;
  openCreateDialog: () => void;
  openEditDialog: (association: ModelWithProvider) => void;
  setEditingAssociation: (association: ModelWithProvider | null) => void;
  setDeleteId: (id: number | null) => void;

  setTestDialogOpen: (open: boolean) => void;
  setSelectedTestId: (id: number | null) => void;
  setTestType: (type: TestType) => void;
  setIsSubmitting: (loading: boolean) => void;

  setModelListDialogOpen: (open: boolean) => void;
  setModelSearchKeyword: (keyword: string) => void;
  setSelectedProviderModels: (next: Updater<ProviderModelSelection[]>) => void;
  setCollapsedProviders: (next: Updater<Record<number, boolean>>) => void;
  setSelectedAssociationIds: (next: Updater<number[]>) => void;

  setBatchDeleteDialogOpen: (open: boolean) => void;
  setBatchDeleting: (loading: boolean) => void;
  setBatchUpdatingStatus: (loading: boolean) => void;
  setBatchActionSheetOpen: (open: boolean) => void;
  setBatchCapabilitiesDialogOpen: (open: boolean) => void;
  setBatchUpdatingCapabilities: (loading: boolean) => void;

  setSearchKeyword: (keyword: string) => void;
  setSelectedProviderType: (value: string) => void;
  setSelectedProviderFilter: (value: string) => void;
  setSelectedStatusFilter: (value: string) => void;
  setFilterPanelOpen: (open: boolean) => void;
  setOperationScope: (scope: "current" | "all") => void;

  setPreviewDialogOpen: (open: boolean) => void;
  setPreviewType: (type: "associate" | "clean") => void;
  setExecuting: (loading: boolean) => void;

  setTemplateEditorOpen: (open: boolean) => void;
  setTemplateLoading: (loading: boolean) => void;
  setTemplateNewItem: (value: string) => void;

  setResettingWeights: (loading: boolean) => void;
  setResettingPriorities: (loading: boolean) => void;
  setEnablingAssociations: (loading: boolean) => void;

  setBatchTesting: (loading: boolean) => void;
  setBatchTestProgress: (next: Updater<BatchTestProgress>) => void;
  setAssociationTestResults: (next: Updater<Record<number, AssociationBatchTestResult>>) => void;

  setReactTestResult: (next: Updater<ReactTestResultState>) => void;

  setBlacklistDialogOpen: (open: boolean) => void;
  setBlacklistedIds: (next: Updater<number[]>) => void;
  setBlacklistLoading: (loading: boolean) => void;
  setBlacklistSaving: (loading: boolean) => void;
  setBlacklistSearchTerm: (value: string) => void;
  setBlacklistFilter: (value: BlacklistFilter) => void;

  resetTransient: () => void;
};

function persistPreferences(next: ModelProvidersPagePreferences): void {
  writeModelProvidersPagePreferences(next);
}

function buildPreferences(current: ModelProvidersPageState, overrides: Partial<ModelProvidersPagePreferences> = {}): ModelProvidersPagePreferences {
  return {
    searchKeyword: overrides.searchKeyword ?? current.searchKeyword,
    selectedProviderType: overrides.selectedProviderType ?? current.selectedProviderType,
    selectedProviderFilter: overrides.selectedProviderFilter ?? current.selectedProviderFilter,
    selectedStatusFilter: overrides.selectedStatusFilter ?? current.selectedStatusFilter,
    filterPanelOpen: overrides.filterPanelOpen ?? current.filterPanelOpen,
    operationScope: overrides.operationScope ?? current.operationScope,
  };
}

export const modelProvidersPageStore = createStore<ModelProvidersPageState>()((set, get) => ({
  open: false,
  editingAssociation: null,
  deleteId: null,

  testDialogOpen: false,
  selectedTestId: null,
  testType: "connectivity",
  isSubmitting: false,

  modelListDialogOpen: false,
  modelSearchKeyword: "",
  selectedProviderModels: [],
  collapsedProviders: {},
  selectedAssociationIds: [],

  batchDeleteDialogOpen: false,
  batchDeleting: false,
  batchUpdatingStatus: false,
  batchActionSheetOpen: false,
  batchCapabilitiesDialogOpen: false,
  batchUpdatingCapabilities: false,

  searchKeyword: preferences.searchKeyword,
  selectedProviderType: preferences.selectedProviderType,
  selectedProviderFilter: preferences.selectedProviderFilter,
  selectedStatusFilter: preferences.selectedStatusFilter,
  filterPanelOpen: preferences.filterPanelOpen,
  operationScope: preferences.operationScope,

  previewDialogOpen: false,
  previewType: "associate",
  executing: false,

  templateEditorOpen: false,
  templateLoading: false,
  templateNewItem: "",

  resettingWeights: false,
  resettingPriorities: false,
  enablingAssociations: false,

  batchTesting: false,
  batchTestProgress: getInitialBatchTestProgress(),
  associationTestResults: {},

  reactTestResult: getInitialReactTestResult(),

  blacklistDialogOpen: false,
  blacklistedIds: [],
  blacklistLoading: false,
  blacklistSaving: false,
  blacklistSearchTerm: "",
  blacklistFilter: "all",

  setOpen: (open: boolean) => set(open ? { open: true } : { open: false, editingAssociation: null }),
  openCreateDialog: () => set({ open: true, editingAssociation: null }),
  openEditDialog: (association: ModelWithProvider) => set({ open: true, editingAssociation: association }),
  setEditingAssociation: (association: ModelWithProvider | null) => set({ editingAssociation: association }),
  setDeleteId: (id: number | null) => set({ deleteId: id }),

  setTestDialogOpen: (open: boolean) => set({ testDialogOpen: open }),
  setSelectedTestId: (id: number | null) => set({ selectedTestId: id }),
  setTestType: (type: TestType) => set({ testType: type }),
  setIsSubmitting: (loading: boolean) => set({ isSubmitting: loading }),

  setModelListDialogOpen: (open: boolean) => set({ modelListDialogOpen: open }),
  setModelSearchKeyword: (keyword: string) => set({ modelSearchKeyword: keyword }),
  setSelectedProviderModels: (next: Updater<ProviderModelSelection[]>) =>
    set((state) => ({ selectedProviderModels: resolveUpdater(next, state.selectedProviderModels) })),
  setCollapsedProviders: (next: Updater<Record<number, boolean>>) =>
    set((state) => ({ collapsedProviders: resolveUpdater(next, state.collapsedProviders) })),
  setSelectedAssociationIds: (next: Updater<number[]>) =>
    set((state) => ({ selectedAssociationIds: resolveUpdater(next, state.selectedAssociationIds) })),

  setBatchDeleteDialogOpen: (open: boolean) => set({ batchDeleteDialogOpen: open }),
  setBatchDeleting: (loading: boolean) => set({ batchDeleting: loading }),
  setBatchUpdatingStatus: (loading: boolean) => set({ batchUpdatingStatus: loading }),
  setBatchActionSheetOpen: (open: boolean) => set({ batchActionSheetOpen: open }),
  setBatchCapabilitiesDialogOpen: (open: boolean) => set({ batchCapabilitiesDialogOpen: open }),
  setBatchUpdatingCapabilities: (loading: boolean) => set({ batchUpdatingCapabilities: loading }),

  setSearchKeyword: (keyword: string) => {
    set({ searchKeyword: keyword });
    persistPreferences(buildPreferences(get(), { searchKeyword: keyword }));
  },
  setSelectedProviderType: (value: string) => {
    set({ selectedProviderType: value });
    persistPreferences(buildPreferences(get(), { selectedProviderType: value }));
  },
  setSelectedProviderFilter: (value: string) => {
    set({ selectedProviderFilter: value });
    persistPreferences(buildPreferences(get(), { selectedProviderFilter: value }));
  },
  setSelectedStatusFilter: (value: string) => {
    set({ selectedStatusFilter: value });
    persistPreferences(buildPreferences(get(), { selectedStatusFilter: value }));
  },
  setFilterPanelOpen: (open: boolean) => {
    set({ filterPanelOpen: open });
    persistPreferences(buildPreferences(get(), { filterPanelOpen: open }));
  },
  setOperationScope: (scope: "current" | "all") => {
    set({ operationScope: scope });
    persistPreferences(buildPreferences(get(), { operationScope: scope }));
  },

  setPreviewDialogOpen: (open: boolean) => set({ previewDialogOpen: open }),
  setPreviewType: (type: "associate" | "clean") => set({ previewType: type }),
  setExecuting: (loading: boolean) => set({ executing: loading }),

  setTemplateEditorOpen: (open: boolean) => set({ templateEditorOpen: open }),
  setTemplateLoading: (loading: boolean) => set({ templateLoading: loading }),
  setTemplateNewItem: (value: string) => set({ templateNewItem: value }),

  setResettingWeights: (loading: boolean) => set({ resettingWeights: loading }),
  setResettingPriorities: (loading: boolean) => set({ resettingPriorities: loading }),
  setEnablingAssociations: (loading: boolean) => set({ enablingAssociations: loading }),

  setBatchTesting: (loading: boolean) => set({ batchTesting: loading }),
  setBatchTestProgress: (next: Updater<BatchTestProgress>) => set((state) => ({ batchTestProgress: resolveUpdater(next, state.batchTestProgress) })),
  setAssociationTestResults: (next: Updater<Record<number, AssociationBatchTestResult>>) =>
    set((state) => ({ associationTestResults: resolveUpdater(next, state.associationTestResults) })),

  setReactTestResult: (next: Updater<ReactTestResultState>) => set((state) => ({ reactTestResult: resolveUpdater(next, state.reactTestResult) })),

  setBlacklistDialogOpen: (open: boolean) => set({ blacklistDialogOpen: open }),
  setBlacklistedIds: (next: Updater<number[]>) => set((state) => ({ blacklistedIds: resolveUpdater(next, state.blacklistedIds) })),
  setBlacklistLoading: (loading: boolean) => set({ blacklistLoading: loading }),
  setBlacklistSaving: (loading: boolean) => set({ blacklistSaving: loading }),
  setBlacklistSearchTerm: (value: string) => set({ blacklistSearchTerm: value }),
  setBlacklistFilter: (value: BlacklistFilter) => set({ blacklistFilter: value }),

  resetTransient: () =>
    set({
      open: false,
      editingAssociation: null,
      deleteId: null,
      testDialogOpen: false,
      selectedTestId: null,
      testType: "connectivity",
      isSubmitting: false,
      modelListDialogOpen: false,
      modelSearchKeyword: "",
      selectedProviderModels: [],
      collapsedProviders: {},
      selectedAssociationIds: [],
      batchDeleteDialogOpen: false,
      batchDeleting: false,
      batchUpdatingStatus: false,
      batchActionSheetOpen: false,
      batchCapabilitiesDialogOpen: false,
      batchUpdatingCapabilities: false,
      previewDialogOpen: false,
      previewType: "associate",
      executing: false,
      templateEditorOpen: false,
      templateLoading: false,
      templateNewItem: "",
      resettingWeights: false,
      resettingPriorities: false,
      enablingAssociations: false,
      batchTesting: false,
      batchTestProgress: getInitialBatchTestProgress(),
      associationTestResults: {},
      reactTestResult: getInitialReactTestResult(),
      blacklistDialogOpen: false,
      blacklistLoading: false,
      blacklistSaving: false,
      blacklistSearchTerm: "",
      blacklistFilter: "all",
    }),
}));
