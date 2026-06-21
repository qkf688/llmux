import type { ModelsPageState } from "@/stores/models/page-store";

export const selectModelsBatchDeleting = (state: ModelsPageState) => state.batchDeleting;
export const selectModelsBatchUpdating = (state: ModelsPageState) => state.batchUpdating;

export const selectModelsSearchQuery = (state: ModelsPageState) => state.searchQuery;
export const selectModelsSelectedProviderId = (state: ModelsPageState) => state.selectedProviderId;
export const selectModelsModelSearchQuery = (state: ModelsPageState) => state.modelSearchQuery;

export const selectModelsFormDialogOpen = (state: ModelsPageState) => state.formDialogOpen;
export const selectModelsEditingModel = (state: ModelsPageState) => state.editingModel;
export const selectModelsDeletingModel = (state: ModelsPageState) => state.deletingModel;
export const selectModelsSelectedIds = (state: ModelsPageState) => state.selectedIds;

export const selectModelsBatchDeleteDialogOpen = (state: ModelsPageState) => state.batchDeleteDialogOpen;
export const selectModelsBatchSettingsDialogOpen = (state: ModelsPageState) => state.batchSettingsDialogOpen;
export const selectModelsModelPickerOpen = (state: ModelsPageState) => state.modelPickerOpen;
export const selectModelsCollapsedProviders = (state: ModelsPageState) => state.collapsedProviders;

export const selectSetModelsBatchDeleting = (state: ModelsPageState) => state.setBatchDeleting;
export const selectSetModelsBatchUpdating = (state: ModelsPageState) => state.setBatchUpdating;

export const selectSetModelsSearchQuery = (state: ModelsPageState) => state.setSearchQuery;
export const selectSetModelsSelectedProviderId = (state: ModelsPageState) => state.setSelectedProviderId;
export const selectSetModelsModelSearchQuery = (state: ModelsPageState) => state.setModelSearchQuery;

export const selectSetModelsFormDialogOpen = (state: ModelsPageState) => state.setFormDialogOpen;
export const selectSetModelsEditingModel = (state: ModelsPageState) => state.setEditingModel;
export const selectSetModelsDeletingModel = (state: ModelsPageState) => state.setDeletingModel;
export const selectSetModelsSelectedIds = (state: ModelsPageState) => state.setSelectedIds;

export const selectSetModelsBatchDeleteDialogOpen = (state: ModelsPageState) => state.setBatchDeleteDialogOpen;
export const selectSetModelsBatchSettingsDialogOpen = (state: ModelsPageState) => state.setBatchSettingsDialogOpen;
export const selectSetModelsModelPickerOpen = (state: ModelsPageState) => state.setModelPickerOpen;
export const selectSetModelsCollapsedProviders = (state: ModelsPageState) => state.setCollapsedProviders;

export const selectResetModelsTransient = (state: ModelsPageState) => state.resetTransient;

