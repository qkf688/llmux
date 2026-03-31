import type { VirtualModelsPageState } from "@/stores/virtual-models/page-store";

export const selectVirtualModelsLoading = (state: VirtualModelsPageState) => state.loading;
export const selectVirtualModelsVirtualModels = (state: VirtualModelsPageState) => state.virtualModels;
export const selectVirtualModelsRealModels = (state: VirtualModelsPageState) => state.realModels;
export const selectVirtualModelsProviders = (state: VirtualModelsPageState) => state.providers;
export const selectVirtualModelsBlacklistedProviders = (state: VirtualModelsPageState) => state.blacklistedProviders;

export const selectVirtualModelsModelDialogOpen = (state: VirtualModelsPageState) => state.modelDialogOpen;
export const selectVirtualModelsEditingModel = (state: VirtualModelsPageState) => state.editingModel;
export const selectVirtualModelsModelToDeleteId = (state: VirtualModelsPageState) => state.modelToDeleteId;

export const selectVirtualModelsMappingsDialogOpen = (state: VirtualModelsPageState) => state.mappingsDialogOpen;
export const selectVirtualModelsCurrentVirtualModel = (state: VirtualModelsPageState) => state.currentVirtualModel;
export const selectVirtualModelsMappings = (state: VirtualModelsPageState) => state.mappings;
export const selectVirtualModelsMappingSearchQuery = (state: VirtualModelsPageState) => state.mappingSearchQuery;
export const selectVirtualModelsSelectedMappingIds = (state: VirtualModelsPageState) => state.selectedMappingIds;
export const selectVirtualModelsMappingBatchDeleteDialogOpen = (state: VirtualModelsPageState) =>
  state.mappingBatchDeleteDialogOpen;

export const selectVirtualModelsMappingFormDialogOpen = (state: VirtualModelsPageState) => state.mappingFormDialogOpen;
export const selectVirtualModelsEditingMapping = (state: VirtualModelsPageState) => state.editingMapping;

export const selectVirtualModelsMappingBatchDialogOpen = (state: VirtualModelsPageState) => state.mappingBatchDialogOpen;
export const selectVirtualModelsSelectedModelIds = (state: VirtualModelsPageState) => state.selectedModelIds;
export const selectVirtualModelsBatchPriority = (state: VirtualModelsPageState) => state.batchPriority;
export const selectVirtualModelsBatchWeight = (state: VirtualModelsPageState) => state.batchWeight;
export const selectVirtualModelsBatchEnabled = (state: VirtualModelsPageState) => state.batchEnabled;
export const selectVirtualModelsModelSearchQuery = (state: VirtualModelsPageState) => state.modelSearchQuery;

export const selectVirtualModelsBlacklistDialogOpen = (state: VirtualModelsPageState) => state.blacklistDialogOpen;
export const selectVirtualModelsProviderSelectorDialogOpen = (state: VirtualModelsPageState) =>
  state.providerSelectorDialogOpen;
export const selectVirtualModelsSelectedProviderIds = (state: VirtualModelsPageState) => state.selectedProviderIds;
export const selectVirtualModelsProviderSearchQuery = (state: VirtualModelsPageState) => state.providerSearchQuery;

export const selectSetVirtualModelsLoading = (state: VirtualModelsPageState) => state.setLoading;
export const selectSetVirtualModelsVirtualModels = (state: VirtualModelsPageState) => state.setVirtualModels;
export const selectSetVirtualModelsRealModels = (state: VirtualModelsPageState) => state.setRealModels;
export const selectSetVirtualModelsProviders = (state: VirtualModelsPageState) => state.setProviders;
export const selectSetVirtualModelsBlacklistedProviders = (state: VirtualModelsPageState) => state.setBlacklistedProviders;
export const selectSetVirtualModelsMappings = (state: VirtualModelsPageState) => state.setMappings;

export const selectSetVirtualModelsModelDialogOpen = (state: VirtualModelsPageState) => state.setModelDialogOpen;
export const selectSetVirtualModelsEditingModel = (state: VirtualModelsPageState) => state.setEditingModel;
export const selectSetVirtualModelsModelToDeleteId = (state: VirtualModelsPageState) => state.setModelToDeleteId;

export const selectSetVirtualModelsMappingsDialogOpen = (state: VirtualModelsPageState) => state.setMappingsDialogOpen;
export const selectSetVirtualModelsCurrentVirtualModel = (state: VirtualModelsPageState) => state.setCurrentVirtualModel;
export const selectSetVirtualModelsMappingSearchQuery = (state: VirtualModelsPageState) => state.setMappingSearchQuery;
export const selectSetVirtualModelsSelectedMappingIds = (state: VirtualModelsPageState) => state.setSelectedMappingIds;
export const selectSetVirtualModelsMappingBatchDeleteDialogOpen = (state: VirtualModelsPageState) =>
  state.setMappingBatchDeleteDialogOpen;

export const selectSetVirtualModelsMappingFormDialogOpen = (state: VirtualModelsPageState) =>
  state.setMappingFormDialogOpen;
export const selectSetVirtualModelsEditingMapping = (state: VirtualModelsPageState) => state.setEditingMapping;

export const selectSetVirtualModelsMappingBatchDialogOpen = (state: VirtualModelsPageState) =>
  state.setMappingBatchDialogOpen;
export const selectSetVirtualModelsSelectedModelIds = (state: VirtualModelsPageState) => state.setSelectedModelIds;
export const selectSetVirtualModelsBatchPriority = (state: VirtualModelsPageState) => state.setBatchPriority;
export const selectSetVirtualModelsBatchWeight = (state: VirtualModelsPageState) => state.setBatchWeight;
export const selectSetVirtualModelsBatchEnabled = (state: VirtualModelsPageState) => state.setBatchEnabled;
export const selectSetVirtualModelsModelSearchQuery = (state: VirtualModelsPageState) => state.setModelSearchQuery;

export const selectSetVirtualModelsBlacklistDialogOpen = (state: VirtualModelsPageState) => state.setBlacklistDialogOpen;
export const selectSetVirtualModelsProviderSelectorDialogOpen = (state: VirtualModelsPageState) =>
  state.setProviderSelectorDialogOpen;
export const selectSetVirtualModelsSelectedProviderIds = (state: VirtualModelsPageState) => state.setSelectedProviderIds;
export const selectSetVirtualModelsProviderSearchQuery = (state: VirtualModelsPageState) => state.setProviderSearchQuery;

export const selectResetVirtualModelsTransient = (state: VirtualModelsPageState) => state.resetTransient;
