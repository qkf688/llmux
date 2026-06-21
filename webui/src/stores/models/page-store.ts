import { createStore } from "zustand/vanilla";
import type { Model } from "@/lib/api";
import { readModelsPagePreferences, writeModelsPagePreferences } from "@/stores/models/persist";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

export type ModelsPageState = {
  batchDeleting: boolean;
  batchUpdating: boolean;

  searchQuery: string;
  selectedProviderId: string;
  modelSearchQuery: string;

  formDialogOpen: boolean;
  editingModel: Model | null;
  deletingModel: Model | null;
  selectedIds: number[];

  batchDeleteDialogOpen: boolean;
  batchSettingsDialogOpen: boolean;
  modelPickerOpen: boolean;
  collapsedProviders: Record<number, boolean>;

  setBatchDeleting: (loading: boolean) => void;
  setBatchUpdating: (loading: boolean) => void;

  setSearchQuery: (query: string) => void;
  setSelectedProviderId: (providerId: string) => void;
  setModelSearchQuery: (query: string) => void;

  setFormDialogOpen: (open: boolean) => void;
  openCreateDialog: () => void;
  openEditDialog: (model: Model) => void;
  setEditingModel: (model: Model | null) => void;

  openDeleteDialog: (model: Model) => void;
  setDeletingModel: (model: Model | null) => void;
  clearDeleteDialog: () => void;

  setSelectedIds: (ids: Updater<number[]>) => void;
  clearSelectedIds: () => void;

  setBatchDeleteDialogOpen: (open: boolean) => void;
  setBatchSettingsDialogOpen: (open: boolean) => void;

  setModelPickerOpen: (open: boolean) => void;
  setCollapsedProviders: (next: Updater<Record<number, boolean>>) => void;

  resetTransient: () => void;
};

const preferences = readModelsPagePreferences();

export const modelsPageStore = createStore<ModelsPageState>()((set, get) => ({
  batchDeleting: false,
  batchUpdating: false,

  searchQuery: preferences.searchQuery,
  selectedProviderId: preferences.selectedProviderId,
  modelSearchQuery: preferences.modelSearchQuery,

  formDialogOpen: false,
  editingModel: null,
  deletingModel: null,
  selectedIds: [],

  batchDeleteDialogOpen: false,
  batchSettingsDialogOpen: false,
  modelPickerOpen: false,
  collapsedProviders: {},

  setBatchDeleting: (loading: boolean) => set({ batchDeleting: loading }),
  setBatchUpdating: (loading: boolean) => set({ batchUpdating: loading }),

  setSearchQuery: (query: string) => {
    set({ searchQuery: query });
    const current = get();
    writeModelsPagePreferences({ searchQuery: query, selectedProviderId: current.selectedProviderId, modelSearchQuery: current.modelSearchQuery });
  },
  setSelectedProviderId: (providerId: string) => {
    set({ selectedProviderId: providerId });
    const current = get();
    writeModelsPagePreferences({ searchQuery: current.searchQuery, selectedProviderId: providerId, modelSearchQuery: current.modelSearchQuery });
  },
  setModelSearchQuery: (query: string) => {
    set({ modelSearchQuery: query });
    const current = get();
    writeModelsPagePreferences({ searchQuery: current.searchQuery, selectedProviderId: current.selectedProviderId, modelSearchQuery: query });
  },

  setFormDialogOpen: (open: boolean) => set(open ? { formDialogOpen: true } : { formDialogOpen: false, editingModel: null }),
  openCreateDialog: () => set({ formDialogOpen: true, editingModel: null }),
  openEditDialog: (model: Model) => set({ formDialogOpen: true, editingModel: model }),
  setEditingModel: (model: Model | null) => set({ editingModel: model }),

  openDeleteDialog: (model: Model) => set({ deletingModel: model }),
  setDeletingModel: (model: Model | null) => set({ deletingModel: model }),
  clearDeleteDialog: () => set({ deletingModel: null }),

  setSelectedIds: (ids: Updater<number[]>) => set((state) => ({ selectedIds: resolveUpdater(ids, state.selectedIds) })),
  clearSelectedIds: () => set({ selectedIds: [] }),

  setBatchDeleteDialogOpen: (open: boolean) => set({ batchDeleteDialogOpen: open }),
  setBatchSettingsDialogOpen: (open: boolean) => set({ batchSettingsDialogOpen: open }),

  setModelPickerOpen: (open: boolean) => set({ modelPickerOpen: open }),
  setCollapsedProviders: (next: Updater<Record<number, boolean>>) =>
    set((state) => ({ collapsedProviders: resolveUpdater(next, state.collapsedProviders) })),

  resetTransient: () =>
    set({
      batchDeleting: false,
      batchUpdating: false,
      formDialogOpen: false,
      editingModel: null,
      deletingModel: null,
      selectedIds: [],
      batchDeleteDialogOpen: false,
      batchSettingsDialogOpen: false,
      modelPickerOpen: false,
    }),
}));
