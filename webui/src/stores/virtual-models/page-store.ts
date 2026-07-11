import { createStore } from "zustand/vanilla";
import type { VirtualModel, VirtualModelMapping } from "@/lib/api";
import { type Updater, resolveUpdater } from "@/stores/core/updater";
import { readVirtualModelsPagePreferences, writeVirtualModelsPagePreferences } from "@/stores/virtual-models/persist";
import type { VirtualModelsPagePreferences } from "@/stores/virtual-models/types";

type PreferencesState = {
  batchPriority: number;
  batchWeight: number;
  batchEnabled: boolean;
  modelSearchQuery: string;
  providerSearchQuery: string;
};

function persistPreferences(get: () => PreferencesState, next: Partial<VirtualModelsPagePreferences>): void {
  const current = get();
  writeVirtualModelsPagePreferences({
    batchPriority: next.batchPriority ?? current.batchPriority,
    batchWeight: next.batchWeight ?? current.batchWeight,
    batchEnabled: next.batchEnabled ?? current.batchEnabled,
    modelSearchQuery: next.modelSearchQuery ?? current.modelSearchQuery,
    providerSearchQuery: next.providerSearchQuery ?? current.providerSearchQuery,
  });
}

const preferences = readVirtualModelsPagePreferences();

export type VirtualModelsPageState = {
  modelDialogOpen: boolean;
  editingModel: VirtualModel | null;
  modelToDeleteId: number | null;

  mappingsDialogOpen: boolean;
  currentVirtualModel: VirtualModel | null;
  mappingSearchQuery: string;
  selectedMappingIds: Set<number>;
  mappingBatchDeleteDialogOpen: boolean;

  mappingFormDialogOpen: boolean;
  editingMapping: VirtualModelMapping | null;

  mappingBatchDialogOpen: boolean;
  selectedModelIds: number[];
  batchPriority: number;
  batchWeight: number;
  batchEnabled: boolean;
  modelSearchQuery: string;

  blacklistDialogOpen: boolean;
  providerSelectorDialogOpen: boolean;
  selectedProviderIds: number[];
  providerSearchQuery: string;

  setModelDialogOpen: (open: boolean) => void;
  setEditingModel: (model: VirtualModel | null) => void;
  setModelToDeleteId: (id: number | null) => void;

  setMappingsDialogOpen: (open: boolean) => void;
  setCurrentVirtualModel: (model: VirtualModel | null) => void;
  setMappingSearchQuery: (query: string) => void;
  setSelectedMappingIds: (next: Updater<Set<number>>) => void;
  setMappingBatchDeleteDialogOpen: (open: boolean) => void;

  setMappingFormDialogOpen: (open: boolean) => void;
  setEditingMapping: (mapping: VirtualModelMapping | null) => void;

  setMappingBatchDialogOpen: (open: boolean) => void;
  setSelectedModelIds: (next: Updater<number[]>) => void;
  setBatchPriority: (priority: number) => void;
  setBatchWeight: (weight: number) => void;
  setBatchEnabled: (enabled: boolean) => void;
  setModelSearchQuery: (query: string) => void;

  setBlacklistDialogOpen: (open: boolean) => void;
  setProviderSelectorDialogOpen: (open: boolean) => void;
  setSelectedProviderIds: (next: Updater<number[]>) => void;
  setProviderSearchQuery: (query: string) => void;

  resetTransient: () => void;
};

export const virtualModelsPageStore = createStore<VirtualModelsPageState>()((set, get) => ({
  modelDialogOpen: false,
  editingModel: null,
  modelToDeleteId: null,

  mappingsDialogOpen: false,
  currentVirtualModel: null,
  mappingSearchQuery: "",
  selectedMappingIds: new Set<number>(),
  mappingBatchDeleteDialogOpen: false,

  mappingFormDialogOpen: false,
  editingMapping: null,

  mappingBatchDialogOpen: false,
  selectedModelIds: [],
  batchPriority: preferences.batchPriority,
  batchWeight: preferences.batchWeight,
  batchEnabled: preferences.batchEnabled,
  modelSearchQuery: preferences.modelSearchQuery,

  blacklistDialogOpen: false,
  providerSelectorDialogOpen: false,
  selectedProviderIds: [],
  providerSearchQuery: preferences.providerSearchQuery,

  setModelDialogOpen: (open: boolean) => set({ modelDialogOpen: open }),
  setEditingModel: (model: VirtualModel | null) => set({ editingModel: model }),
  setModelToDeleteId: (id: number | null) => set({ modelToDeleteId: id }),

  setMappingsDialogOpen: (open: boolean) => set({ mappingsDialogOpen: open }),
  setCurrentVirtualModel: (model: VirtualModel | null) => set({ currentVirtualModel: model }),
  setMappingSearchQuery: (query: string) => set({ mappingSearchQuery: query }),
  setSelectedMappingIds: (next: Updater<Set<number>>) =>
    set((state) => ({ selectedMappingIds: resolveUpdater(next, state.selectedMappingIds) })),
  setMappingBatchDeleteDialogOpen: (open: boolean) => set({ mappingBatchDeleteDialogOpen: open }),

  setMappingFormDialogOpen: (open: boolean) => set({ mappingFormDialogOpen: open }),
  setEditingMapping: (mapping: VirtualModelMapping | null) => set({ editingMapping: mapping }),

  setMappingBatchDialogOpen: (open: boolean) => set({ mappingBatchDialogOpen: open }),
  setSelectedModelIds: (next: Updater<number[]>) =>
    set((state) => ({ selectedModelIds: resolveUpdater(next, state.selectedModelIds) })),
  setBatchPriority: (priority: number) => {
    const current = get();
    if (current.batchPriority === priority) return;
    persistPreferences(get, { batchPriority: priority });
    set({ batchPriority: priority });
  },
  setBatchWeight: (weight: number) => {
    const current = get();
    if (current.batchWeight === weight) return;
    persistPreferences(get, { batchWeight: weight });
    set({ batchWeight: weight });
  },
  setBatchEnabled: (enabled: boolean) => {
    const current = get();
    if (current.batchEnabled === enabled) return;
    persistPreferences(get, { batchEnabled: enabled });
    set({ batchEnabled: enabled });
  },
  setModelSearchQuery: (query: string) => {
    const current = get();
    if (current.modelSearchQuery === query) return;
    persistPreferences(get, { modelSearchQuery: query });
    set({ modelSearchQuery: query });
  },

  setBlacklistDialogOpen: (open: boolean) => set({ blacklistDialogOpen: open }),
  setProviderSelectorDialogOpen: (open: boolean) => set({ providerSelectorDialogOpen: open }),
  setSelectedProviderIds: (next: Updater<number[]>) =>
    set((state) => ({ selectedProviderIds: resolveUpdater(next, state.selectedProviderIds) })),
  setProviderSearchQuery: (query: string) => {
    const current = get();
    if (current.providerSearchQuery === query) return;
    persistPreferences(get, { providerSearchQuery: query });
    set({ providerSearchQuery: query });
  },

  resetTransient: () =>
    set({
      modelDialogOpen: false,
      editingModel: null,
      modelToDeleteId: null,
      mappingsDialogOpen: false,
      currentVirtualModel: null,
      mappingSearchQuery: "",
      selectedMappingIds: new Set<number>(),
      mappingBatchDeleteDialogOpen: false,
      mappingFormDialogOpen: false,
      editingMapping: null,
      mappingBatchDialogOpen: false,
      selectedModelIds: [],
      blacklistDialogOpen: false,
      providerSelectorDialogOpen: false,
      selectedProviderIds: [],
    }),
}));
