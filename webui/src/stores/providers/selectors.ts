import type { ProvidersPageState } from "@/stores/providers/page-store";

export const selectNameFilter = (state: ProvidersPageState) => state.nameFilter;
export const selectDebouncedNameFilter = (state: ProvidersPageState) => state.debouncedNameFilter;
export const selectTypeFilter = (state: ProvidersPageState) => state.typeFilter;

export const selectModelsLoading = (state: ProvidersPageState) => state.modelsLoading;
export const selectAddingModels = (state: ProvidersPageState) => state.addingModels;
export const selectSyncingModels = (state: ProvidersPageState) => state.syncingModels;
export const selectSyncingAll = (state: ProvidersPageState) => state.syncingAll;
export const selectAutoAssociateOnAddEnabled = (state: ProvidersPageState) => state.autoAssociateOnAddEnabled;
export const selectAutoCleanOnDeleteEnabled = (state: ProvidersPageState) => state.autoCleanOnDeleteEnabled;

export const selectShowApiKey = (state: ProvidersPageState) => state.showApiKey;

export const selectProviderDialogOpen = (state: ProvidersPageState) => state.providerDialogOpen;
export const selectEditingProvider = (state: ProvidersPageState) => state.editingProvider;

export const selectModelsOpen = (state: ProvidersPageState) => state.modelsOpen;
export const selectModelsOpenId = (state: ProvidersPageState) => state.modelsOpenId;

export const selectAllModelsOpen = (state: ProvidersPageState) => state.allModelsOpen;
export const selectAllModelsProvider = (state: ProvidersPageState) => state.allModelsProvider;

export const selectAllModelsTypeFilter = (state: ProvidersPageState) => state.allModelsTypeFilter;

export const selectSelectedUpstreamModels = (state: ProvidersPageState) => state.selectedUpstreamModels;
export const selectSelectedAllModels = (state: ProvidersPageState) => state.selectedAllModels;
export const selectAllModelsSearchQuery = (state: ProvidersPageState) => state.allModelsSearchQuery;
export const selectCustomModelInput = (state: ProvidersPageState) => state.customModelInput;

export const selectAllModelsTestResults = (state: ProvidersPageState) => state.allModelsTestResults;
export const selectBatchTesting = (state: ProvidersPageState) => state.batchTesting;
export const selectBatchTestProgress = (state: ProvidersPageState) => state.batchTestProgress;

export const selectUpstreamTestResults = (state: ProvidersPageState) => state.upstreamTestResults;
export const selectUpstreamBatchTesting = (state: ProvidersPageState) => state.upstreamBatchTesting;
export const selectUpstreamBatchTestProgress = (state: ProvidersPageState) => state.upstreamBatchTestProgress;

export const selectSetNameFilter = (state: ProvidersPageState) => state.setNameFilter;
export const selectSetDebouncedNameFilter = (state: ProvidersPageState) => state.setDebouncedNameFilter;
export const selectFlushNameFilter = (state: ProvidersPageState) => state.flushNameFilter;
export const selectSetTypeFilter = (state: ProvidersPageState) => state.setTypeFilter;

export const selectSetModelsLoading = (state: ProvidersPageState) => state.setModelsLoading;
export const selectSetAddingModels = (state: ProvidersPageState) => state.setAddingModels;
export const selectSetSyncingModels = (state: ProvidersPageState) => state.setSyncingModels;
export const selectSetSyncingAll = (state: ProvidersPageState) => state.setSyncingAll;
export const selectSetAutoAssociateOnAddEnabled = (state: ProvidersPageState) => state.setAutoAssociateOnAddEnabled;
export const selectSetAutoCleanOnDeleteEnabled = (state: ProvidersPageState) => state.setAutoCleanOnDeleteEnabled;

export const selectSetShowApiKey = (state: ProvidersPageState) => state.setShowApiKey;
export const selectToggleShowApiKey = (state: ProvidersPageState) => state.toggleShowApiKey;

export const selectSetProviderDialogOpen = (state: ProvidersPageState) => state.setProviderDialogOpen;
export const selectSetEditingProvider = (state: ProvidersPageState) => state.setEditingProvider;
export const selectOpenCreateProvider = (state: ProvidersPageState) => state.openCreateProvider;
export const selectOpenEditProvider = (state: ProvidersPageState) => state.openEditProvider;

export const selectOpenProviderModels = (state: ProvidersPageState) => state.openProviderModels;
export const selectSetModelsOpen = (state: ProvidersPageState) => state.setModelsOpen;
export const selectSetModelsOpenId = (state: ProvidersPageState) => state.setModelsOpenId;

export const selectOpenAllModels = (state: ProvidersPageState) => state.openAllModels;
export const selectSetAllModelsOpen = (state: ProvidersPageState) => state.setAllModelsOpen;
export const selectSetAllModelsProvider = (state: ProvidersPageState) => state.setAllModelsProvider;

export const selectSetSelectedUpstreamModels = (state: ProvidersPageState) => state.setSelectedUpstreamModels;
export const selectSetSelectedAllModels = (state: ProvidersPageState) => state.setSelectedAllModels;
export const selectSetAllModelsTypeFilter = (state: ProvidersPageState) => state.setAllModelsTypeFilter;
export const selectSetAllModelsSearchQuery = (state: ProvidersPageState) => state.setAllModelsSearchQuery;
export const selectSetCustomModelInput = (state: ProvidersPageState) => state.setCustomModelInput;

export const selectSetAllModelsTestResults = (state: ProvidersPageState) => state.setAllModelsTestResults;
export const selectSetBatchTesting = (state: ProvidersPageState) => state.setBatchTesting;
export const selectSetBatchTestProgress = (state: ProvidersPageState) => state.setBatchTestProgress;

export const selectSetUpstreamTestResults = (state: ProvidersPageState) => state.setUpstreamTestResults;
export const selectSetUpstreamBatchTesting = (state: ProvidersPageState) => state.setUpstreamBatchTesting;
export const selectSetUpstreamBatchTestProgress = (state: ProvidersPageState) => state.setUpstreamBatchTestProgress;

export const selectResetProvidersTransient = (state: ProvidersPageState) => state.resetTransient;
