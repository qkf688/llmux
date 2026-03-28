import type { ModelProvidersPageState } from "@/stores/model-providers/page-store";

export const selectModelProvidersLoading = (state: ModelProvidersPageState) => state.loading;
export const selectModelProvidersOpen = (state: ModelProvidersPageState) => state.open;
export const selectModelProvidersEditingAssociation = (state: ModelProvidersPageState) => state.editingAssociation;
export const selectModelProvidersDeleteId = (state: ModelProvidersPageState) => state.deleteId;

export const selectModelProvidersTestDialogOpen = (state: ModelProvidersPageState) => state.testDialogOpen;
export const selectModelProvidersSelectedTestId = (state: ModelProvidersPageState) => state.selectedTestId;
export const selectModelProvidersTestType = (state: ModelProvidersPageState) => state.testType;
export const selectModelProvidersIsSubmitting = (state: ModelProvidersPageState) => state.isSubmitting;

export const selectModelProvidersLoadingProviderModels = (state: ModelProvidersPageState) => state.loadingProviderModels;
export const selectModelProvidersModelListDialogOpen = (state: ModelProvidersPageState) => state.modelListDialogOpen;
export const selectModelProvidersModelSearchKeyword = (state: ModelProvidersPageState) => state.modelSearchKeyword;
export const selectModelProvidersSelectedProviderModels = (state: ModelProvidersPageState) => state.selectedProviderModels;
export const selectModelProvidersCollapsedProviders = (state: ModelProvidersPageState) => state.collapsedProviders;
export const selectModelProvidersSelectedAssociationIds = (state: ModelProvidersPageState) => state.selectedAssociationIds;

export const selectModelProvidersBatchDeleteDialogOpen = (state: ModelProvidersPageState) => state.batchDeleteDialogOpen;
export const selectModelProvidersBatchDeleting = (state: ModelProvidersPageState) => state.batchDeleting;
export const selectModelProvidersBatchUpdatingStatus = (state: ModelProvidersPageState) => state.batchUpdatingStatus;
export const selectModelProvidersBatchActionSheetOpen = (state: ModelProvidersPageState) => state.batchActionSheetOpen;
export const selectModelProvidersBatchCapabilitiesDialogOpen = (state: ModelProvidersPageState) => state.batchCapabilitiesDialogOpen;
export const selectModelProvidersBatchUpdatingCapabilities = (state: ModelProvidersPageState) => state.batchUpdatingCapabilities;

export const selectModelProvidersSearchKeyword = (state: ModelProvidersPageState) => state.searchKeyword;
export const selectModelProvidersSelectedProviderType = (state: ModelProvidersPageState) => state.selectedProviderType;
export const selectModelProvidersSelectedProviderFilter = (state: ModelProvidersPageState) => state.selectedProviderFilter;
export const selectModelProvidersSelectedStatusFilter = (state: ModelProvidersPageState) => state.selectedStatusFilter;
export const selectModelProvidersFilterPanelOpen = (state: ModelProvidersPageState) => state.filterPanelOpen;
export const selectModelProvidersOperationScope = (state: ModelProvidersPageState) => state.operationScope;

export const selectModelProvidersPreviewDialogOpen = (state: ModelProvidersPageState) => state.previewDialogOpen;
export const selectModelProvidersPreviewType = (state: ModelProvidersPageState) => state.previewType;
export const selectModelProvidersExecuting = (state: ModelProvidersPageState) => state.executing;

export const selectModelProvidersTemplateEditorOpen = (state: ModelProvidersPageState) => state.templateEditorOpen;
export const selectModelProvidersTemplateLoading = (state: ModelProvidersPageState) => state.templateLoading;
export const selectModelProvidersTemplateNewItem = (state: ModelProvidersPageState) => state.templateNewItem;

export const selectModelProvidersResettingWeights = (state: ModelProvidersPageState) => state.resettingWeights;
export const selectModelProvidersResettingPriorities = (state: ModelProvidersPageState) => state.resettingPriorities;
export const selectModelProvidersEnablingAssociations = (state: ModelProvidersPageState) => state.enablingAssociations;

export const selectModelProvidersBatchTesting = (state: ModelProvidersPageState) => state.batchTesting;
export const selectModelProvidersBatchTestProgress = (state: ModelProvidersPageState) => state.batchTestProgress;
export const selectModelProvidersAssociationTestResults = (state: ModelProvidersPageState) => state.associationTestResults;
export const selectModelProvidersReactTestResult = (state: ModelProvidersPageState) => state.reactTestResult;

export const selectModelProvidersBlacklistDialogOpen = (state: ModelProvidersPageState) => state.blacklistDialogOpen;
export const selectModelProvidersBlacklistedIds = (state: ModelProvidersPageState) => state.blacklistedIds;
export const selectModelProvidersBlacklistLoading = (state: ModelProvidersPageState) => state.blacklistLoading;
export const selectModelProvidersBlacklistSaving = (state: ModelProvidersPageState) => state.blacklistSaving;
export const selectModelProvidersBlacklistSearchTerm = (state: ModelProvidersPageState) => state.blacklistSearchTerm;
export const selectModelProvidersBlacklistFilter = (state: ModelProvidersPageState) => state.blacklistFilter;

export const selectSetModelProvidersLoading = (state: ModelProvidersPageState) => state.setLoading;
export const selectSetModelProvidersOpen = (state: ModelProvidersPageState) => state.setOpen;
export const selectSetModelProvidersEditingAssociation = (state: ModelProvidersPageState) => state.setEditingAssociation;
export const selectSetModelProvidersDeleteId = (state: ModelProvidersPageState) => state.setDeleteId;

export const selectSetModelProvidersTestDialogOpen = (state: ModelProvidersPageState) => state.setTestDialogOpen;
export const selectSetModelProvidersSelectedTestId = (state: ModelProvidersPageState) => state.setSelectedTestId;
export const selectSetModelProvidersTestType = (state: ModelProvidersPageState) => state.setTestType;
export const selectSetModelProvidersIsSubmitting = (state: ModelProvidersPageState) => state.setIsSubmitting;

export const selectSetModelProvidersLoadingProviderModels = (state: ModelProvidersPageState) => state.setLoadingProviderModels;
export const selectSetModelProvidersModelListDialogOpen = (state: ModelProvidersPageState) => state.setModelListDialogOpen;
export const selectSetModelProvidersModelSearchKeyword = (state: ModelProvidersPageState) => state.setModelSearchKeyword;
export const selectSetModelProvidersSelectedProviderModels = (state: ModelProvidersPageState) => state.setSelectedProviderModels;
export const selectSetModelProvidersCollapsedProviders = (state: ModelProvidersPageState) => state.setCollapsedProviders;
export const selectSetModelProvidersSelectedAssociationIds = (state: ModelProvidersPageState) => state.setSelectedAssociationIds;

export const selectSetModelProvidersBatchDeleteDialogOpen = (state: ModelProvidersPageState) => state.setBatchDeleteDialogOpen;
export const selectSetModelProvidersBatchDeleting = (state: ModelProvidersPageState) => state.setBatchDeleting;
export const selectSetModelProvidersBatchUpdatingStatus = (state: ModelProvidersPageState) => state.setBatchUpdatingStatus;
export const selectSetModelProvidersBatchActionSheetOpen = (state: ModelProvidersPageState) => state.setBatchActionSheetOpen;
export const selectSetModelProvidersBatchCapabilitiesDialogOpen = (state: ModelProvidersPageState) =>
  state.setBatchCapabilitiesDialogOpen;
export const selectSetModelProvidersBatchUpdatingCapabilities = (state: ModelProvidersPageState) => state.setBatchUpdatingCapabilities;

export const selectSetModelProvidersSearchKeyword = (state: ModelProvidersPageState) => state.setSearchKeyword;
export const selectSetModelProvidersSelectedProviderType = (state: ModelProvidersPageState) => state.setSelectedProviderType;
export const selectSetModelProvidersSelectedProviderFilter = (state: ModelProvidersPageState) => state.setSelectedProviderFilter;
export const selectSetModelProvidersSelectedStatusFilter = (state: ModelProvidersPageState) => state.setSelectedStatusFilter;
export const selectSetModelProvidersFilterPanelOpen = (state: ModelProvidersPageState) => state.setFilterPanelOpen;
export const selectSetModelProvidersOperationScope = (state: ModelProvidersPageState) => state.setOperationScope;

export const selectSetModelProvidersPreviewDialogOpen = (state: ModelProvidersPageState) => state.setPreviewDialogOpen;
export const selectSetModelProvidersPreviewType = (state: ModelProvidersPageState) => state.setPreviewType;
export const selectSetModelProvidersExecuting = (state: ModelProvidersPageState) => state.setExecuting;

export const selectSetModelProvidersTemplateEditorOpen = (state: ModelProvidersPageState) => state.setTemplateEditorOpen;
export const selectSetModelProvidersTemplateLoading = (state: ModelProvidersPageState) => state.setTemplateLoading;
export const selectSetModelProvidersTemplateNewItem = (state: ModelProvidersPageState) => state.setTemplateNewItem;

export const selectSetModelProvidersResettingWeights = (state: ModelProvidersPageState) => state.setResettingWeights;
export const selectSetModelProvidersResettingPriorities = (state: ModelProvidersPageState) => state.setResettingPriorities;
export const selectSetModelProvidersEnablingAssociations = (state: ModelProvidersPageState) => state.setEnablingAssociations;

export const selectSetModelProvidersBatchTesting = (state: ModelProvidersPageState) => state.setBatchTesting;
export const selectSetModelProvidersBatchTestProgress = (state: ModelProvidersPageState) => state.setBatchTestProgress;
export const selectSetModelProvidersAssociationTestResults = (state: ModelProvidersPageState) => state.setAssociationTestResults;
export const selectSetModelProvidersReactTestResult = (state: ModelProvidersPageState) => state.setReactTestResult;

export const selectSetModelProvidersBlacklistDialogOpen = (state: ModelProvidersPageState) => state.setBlacklistDialogOpen;
export const selectSetModelProvidersBlacklistedIds = (state: ModelProvidersPageState) => state.setBlacklistedIds;
export const selectSetModelProvidersBlacklistLoading = (state: ModelProvidersPageState) => state.setBlacklistLoading;
export const selectSetModelProvidersBlacklistSaving = (state: ModelProvidersPageState) => state.setBlacklistSaving;
export const selectSetModelProvidersBlacklistSearchTerm = (state: ModelProvidersPageState) => state.setBlacklistSearchTerm;
export const selectSetModelProvidersBlacklistFilter = (state: ModelProvidersPageState) => state.setBlacklistFilter;

export const selectResetModelProvidersTransient = (state: ModelProvidersPageState) => state.resetTransient;
