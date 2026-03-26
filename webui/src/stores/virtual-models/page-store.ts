import { createStore } from "zustand/vanilla";
import type { Model, Provider, VirtualModel, VirtualModelMapping } from "@/lib/api";
import { DEFAULT_VIRTUAL_MODELS_BATCH } from "@/stores/virtual-models/types";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

export type VirtualModelsPageState = {
  loading: boolean;
  virtualModels: VirtualModel[];
  realModels: Model[];
  providers: Provider[];
  blacklistedProviders: Provider[];

  modelDialogOpen: boolean;
  editingModel: VirtualModel | null;
  modelToDeleteId: number | null;

  mappingsDialogOpen: boolean;
  currentVirtualModel: VirtualModel | null;
  mappings: VirtualModelMapping[];

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

  setLoading: (loading: boolean) => void;
  setVirtualModels: (models: VirtualModel[]) => void;
  setRealModels: (models: Model[]) => void;
  setProviders: (providers: Provider[]) => void;
  setBlacklistedProviders: (providers: Provider[]) => void;
  setMappings: (mappings: VirtualModelMapping[]) => void;

  setModelDialogOpen: (open: boolean) => void;
  setEditingModel: (model: VirtualModel | null) => void;
  setModelToDeleteId: (id: number | null) => void;

  setMappingsDialogOpen: (open: boolean) => void;
  setCurrentVirtualModel: (model: VirtualModel | null) => void;

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

export const virtualModelsPageStore = createStore<VirtualModelsPageState>()((set) => ({
  loading: true,
  virtualModels: [],
  realModels: [],
  providers: [],
  blacklistedProviders: [],

  modelDialogOpen: false,
  editingModel: null,
  modelToDeleteId: null,

  mappingsDialogOpen: false,
  currentVirtualModel: null,
  mappings: [],

  mappingFormDialogOpen: false,
  editingMapping: null,

  mappingBatchDialogOpen: false,
  selectedModelIds: [],
  batchPriority: DEFAULT_VIRTUAL_MODELS_BATCH.priority,
  batchWeight: DEFAULT_VIRTUAL_MODELS_BATCH.weight,
  batchEnabled: DEFAULT_VIRTUAL_MODELS_BATCH.enabled,
  modelSearchQuery: "",

  blacklistDialogOpen: false,
  providerSelectorDialogOpen: false,
  selectedProviderIds: [],
  providerSearchQuery: "",

  setLoading: (loading: boolean) => set({ loading }),
  setVirtualModels: (models: VirtualModel[]) => set({ virtualModels: models }),
  setRealModels: (models: Model[]) => set({ realModels: models }),
  setProviders: (providers: Provider[]) => set({ providers }),
  setBlacklistedProviders: (providers: Provider[]) => set({ blacklistedProviders: providers }),
  setMappings: (mappings: VirtualModelMapping[]) => set({ mappings }),

  setModelDialogOpen: (open: boolean) => set({ modelDialogOpen: open }),
  setEditingModel: (model: VirtualModel | null) => set({ editingModel: model }),
  setModelToDeleteId: (id: number | null) => set({ modelToDeleteId: id }),

  setMappingsDialogOpen: (open: boolean) => set({ mappingsDialogOpen: open }),
  setCurrentVirtualModel: (model: VirtualModel | null) => set({ currentVirtualModel: model }),

  setMappingFormDialogOpen: (open: boolean) => set({ mappingFormDialogOpen: open }),
  setEditingMapping: (mapping: VirtualModelMapping | null) => set({ editingMapping: mapping }),

  setMappingBatchDialogOpen: (open: boolean) => set({ mappingBatchDialogOpen: open }),
  setSelectedModelIds: (next: Updater<number[]>) =>
    set((state) => ({ selectedModelIds: resolveUpdater(next, state.selectedModelIds) })),
  setBatchPriority: (priority: number) => set({ batchPriority: priority }),
  setBatchWeight: (weight: number) => set({ batchWeight: weight }),
  setBatchEnabled: (enabled: boolean) => set({ batchEnabled: enabled }),
  setModelSearchQuery: (query: string) => set({ modelSearchQuery: query }),

  setBlacklistDialogOpen: (open: boolean) => set({ blacklistDialogOpen: open }),
  setProviderSelectorDialogOpen: (open: boolean) => set({ providerSelectorDialogOpen: open }),
  setSelectedProviderIds: (next: Updater<number[]>) =>
    set((state) => ({ selectedProviderIds: resolveUpdater(next, state.selectedProviderIds) })),
  setProviderSearchQuery: (query: string) => set({ providerSearchQuery: query }),

  resetTransient: () =>
    set({
      modelDialogOpen: false,
      editingModel: null,
      modelToDeleteId: null,
      mappingsDialogOpen: false,
      currentVirtualModel: null,
      mappings: [],
      mappingFormDialogOpen: false,
      editingMapping: null,
      mappingBatchDialogOpen: false,
      selectedModelIds: [],
      batchPriority: DEFAULT_VIRTUAL_MODELS_BATCH.priority,
      batchWeight: DEFAULT_VIRTUAL_MODELS_BATCH.weight,
      batchEnabled: DEFAULT_VIRTUAL_MODELS_BATCH.enabled,
      modelSearchQuery: "",
      blacklistDialogOpen: false,
      providerSelectorDialogOpen: false,
      selectedProviderIds: [],
      providerSearchQuery: "",
    }),
}));

