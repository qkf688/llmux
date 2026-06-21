import {
  selectModelProvidersAssociationTestResults,
  selectModelProvidersBatchActionSheetOpen,
  selectModelProvidersBatchDeleteDialogOpen,
  selectModelProvidersBatchDeleting,
  selectModelProvidersBatchCapabilitiesDialogOpen,
  selectModelProvidersBatchTestProgress,
  selectModelProvidersBatchTesting,
  selectModelProvidersBatchUpdatingCapabilities,
  selectModelProvidersBatchUpdatingStatus,
  selectModelProvidersBlacklistDialogOpen,
  selectModelProvidersBlacklistFilter,
  selectModelProvidersBlacklistLoading,
  selectModelProvidersBlacklistSaving,
  selectModelProvidersBlacklistSearchTerm,
  selectModelProvidersBlacklistedIds,
  selectModelProvidersCollapsedProviders,
  selectModelProvidersDeleteId,
  selectModelProvidersEditingAssociation,
  selectModelProvidersEnablingAssociations,
  selectModelProvidersExecuting,
  selectModelProvidersFilterPanelOpen,
  selectModelProvidersIsSubmitting,
  selectModelProvidersModelListDialogOpen,
  selectModelProvidersModelSearchKeyword,
  selectModelProvidersOpen,
  selectModelProvidersOperationScope,
  selectModelProvidersPreviewDialogOpen,
  selectModelProvidersPreviewType,
  selectModelProvidersReactTestResult,
  selectModelProvidersResettingPriorities,
  selectModelProvidersResettingWeights,
  selectModelProvidersSearchKeyword,
  selectModelProvidersSelectedAssociationIds,
  selectModelProvidersSelectedProviderFilter,
  selectModelProvidersSelectedProviderModels,
  selectModelProvidersSelectedProviderType,
  selectModelProvidersSelectedStatusFilter,
  selectModelProvidersSelectedTestId,
  selectModelProvidersTemplateEditorOpen,
  selectModelProvidersTemplateLoading,
  selectModelProvidersTemplateNewItem,
  selectModelProvidersTestDialogOpen,
  selectModelProvidersTestType,
  selectResetModelProvidersTransient,
  selectSetModelProvidersAssociationTestResults,
  selectSetModelProvidersBatchActionSheetOpen,
  selectSetModelProvidersBatchDeleteDialogOpen,
  selectSetModelProvidersBatchDeleting,
  selectSetModelProvidersBatchCapabilitiesDialogOpen,
  selectSetModelProvidersBatchTestProgress,
  selectSetModelProvidersBatchTesting,
  selectSetModelProvidersBatchUpdatingCapabilities,
  selectSetModelProvidersBatchUpdatingStatus,
  selectSetModelProvidersBlacklistDialogOpen,
  selectSetModelProvidersBlacklistFilter,
  selectSetModelProvidersBlacklistLoading,
  selectSetModelProvidersBlacklistSaving,
  selectSetModelProvidersBlacklistSearchTerm,
  selectSetModelProvidersBlacklistedIds,
  selectSetModelProvidersCollapsedProviders,
  selectSetModelProvidersDeleteId,
  selectSetModelProvidersEditingAssociation,
  selectSetModelProvidersEnablingAssociations,
  selectSetModelProvidersExecuting,
  selectSetModelProvidersFilterPanelOpen,
  selectSetModelProvidersIsSubmitting,
  selectSetModelProvidersModelListDialogOpen,
  selectSetModelProvidersModelSearchKeyword,
  selectSetModelProvidersOpen,
  selectSetModelProvidersOperationScope,
  selectSetModelProvidersPreviewDialogOpen,
  selectSetModelProvidersPreviewType,
  selectSetModelProvidersReactTestResult,
  selectSetModelProvidersResettingPriorities,
  selectSetModelProvidersResettingWeights,
  selectSetModelProvidersSearchKeyword,
  selectSetModelProvidersSelectedAssociationIds,
  selectSetModelProvidersSelectedProviderFilter,
  selectSetModelProvidersSelectedProviderModels,
  selectSetModelProvidersSelectedProviderType,
  selectSetModelProvidersSelectedStatusFilter,
  selectSetModelProvidersSelectedTestId,
  selectSetModelProvidersTemplateEditorOpen,
  selectSetModelProvidersTemplateLoading,
  selectSetModelProvidersTemplateNewItem,
  selectSetModelProvidersTestDialogOpen,
  selectSetModelProvidersTestType,
  useModelProvidersPageStore,
} from "@/stores/model-providers";

export function useModelProvidersPageStoreState() {
  const open = useModelProvidersPageStore(selectModelProvidersOpen);
  const setOpen = useModelProvidersPageStore(selectSetModelProvidersOpen);
  const editingAssociation = useModelProvidersPageStore(selectModelProvidersEditingAssociation);
  const setEditingAssociation = useModelProvidersPageStore(selectSetModelProvidersEditingAssociation);
  const deleteId = useModelProvidersPageStore(selectModelProvidersDeleteId);
  const setDeleteId = useModelProvidersPageStore(selectSetModelProvidersDeleteId);

  const testDialogOpen = useModelProvidersPageStore(selectModelProvidersTestDialogOpen);
  const setTestDialogOpen = useModelProvidersPageStore(selectSetModelProvidersTestDialogOpen);
  const selectedTestId = useModelProvidersPageStore(selectModelProvidersSelectedTestId);
  const setSelectedTestId = useModelProvidersPageStore(selectSetModelProvidersSelectedTestId);
  const testType = useModelProvidersPageStore(selectModelProvidersTestType);
  const setTestType = useModelProvidersPageStore(selectSetModelProvidersTestType);
  const reactTestResult = useModelProvidersPageStore(selectModelProvidersReactTestResult);
  const setReactTestResult = useModelProvidersPageStore(selectSetModelProvidersReactTestResult);
  const isSubmitting = useModelProvidersPageStore(selectModelProvidersIsSubmitting);
  const setIsSubmitting = useModelProvidersPageStore(selectSetModelProvidersIsSubmitting);

  const modelListDialogOpen = useModelProvidersPageStore(selectModelProvidersModelListDialogOpen);
  const setModelListDialogOpen = useModelProvidersPageStore(selectSetModelProvidersModelListDialogOpen);
  const modelSearchKeyword = useModelProvidersPageStore(selectModelProvidersModelSearchKeyword);
  const setModelSearchKeyword = useModelProvidersPageStore(selectSetModelProvidersModelSearchKeyword);
  const selectedProviderModels = useModelProvidersPageStore(selectModelProvidersSelectedProviderModels);
  const setSelectedProviderModels = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderModels);
  const selectedAssociationIds = useModelProvidersPageStore(selectModelProvidersSelectedAssociationIds);
  const setSelectedAssociationIds = useModelProvidersPageStore(selectSetModelProvidersSelectedAssociationIds);
  const collapsedProviders = useModelProvidersPageStore(selectModelProvidersCollapsedProviders);
  const setCollapsedProviders = useModelProvidersPageStore(selectSetModelProvidersCollapsedProviders);

  const batchDeleteDialogOpen = useModelProvidersPageStore(selectModelProvidersBatchDeleteDialogOpen);
  const setBatchDeleteDialogOpen = useModelProvidersPageStore(selectSetModelProvidersBatchDeleteDialogOpen);
  const batchDeleting = useModelProvidersPageStore(selectModelProvidersBatchDeleting);
  const setBatchDeleting = useModelProvidersPageStore(selectSetModelProvidersBatchDeleting);
  const batchUpdatingStatus = useModelProvidersPageStore(selectModelProvidersBatchUpdatingStatus);
  const setBatchUpdatingStatus = useModelProvidersPageStore(selectSetModelProvidersBatchUpdatingStatus);
  const batchActionSheetOpen = useModelProvidersPageStore(selectModelProvidersBatchActionSheetOpen);
  const setBatchActionSheetOpen = useModelProvidersPageStore(selectSetModelProvidersBatchActionSheetOpen);
  const batchCapabilitiesDialogOpen = useModelProvidersPageStore(selectModelProvidersBatchCapabilitiesDialogOpen);
  const setBatchCapabilitiesDialogOpen = useModelProvidersPageStore(selectSetModelProvidersBatchCapabilitiesDialogOpen);
  const batchUpdatingCapabilities = useModelProvidersPageStore(selectModelProvidersBatchUpdatingCapabilities);
  const setBatchUpdatingCapabilities = useModelProvidersPageStore(selectSetModelProvidersBatchUpdatingCapabilities);

  const searchKeyword = useModelProvidersPageStore(selectModelProvidersSearchKeyword);
  const setSearchKeyword = useModelProvidersPageStore(selectSetModelProvidersSearchKeyword);
  const selectedProviderType = useModelProvidersPageStore(selectModelProvidersSelectedProviderType);
  const setSelectedProviderType = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderType);
  const selectedProviderFilter = useModelProvidersPageStore(selectModelProvidersSelectedProviderFilter);
  const setSelectedProviderFilter = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderFilter);
  const selectedStatusFilter = useModelProvidersPageStore(selectModelProvidersSelectedStatusFilter);
  const setSelectedStatusFilter = useModelProvidersPageStore(selectSetModelProvidersSelectedStatusFilter);
  const operationScope = useModelProvidersPageStore(selectModelProvidersOperationScope);
  const setOperationScope = useModelProvidersPageStore(selectSetModelProvidersOperationScope);
  const filterPanelOpen = useModelProvidersPageStore(selectModelProvidersFilterPanelOpen);
  const setFilterPanelOpen = useModelProvidersPageStore(selectSetModelProvidersFilterPanelOpen);

  const previewDialogOpen = useModelProvidersPageStore(selectModelProvidersPreviewDialogOpen);
  const setPreviewDialogOpen = useModelProvidersPageStore(selectSetModelProvidersPreviewDialogOpen);
  const previewType = useModelProvidersPageStore(selectModelProvidersPreviewType);
  const setPreviewType = useModelProvidersPageStore(selectSetModelProvidersPreviewType);
  const executing = useModelProvidersPageStore(selectModelProvidersExecuting);
  const setExecuting = useModelProvidersPageStore(selectSetModelProvidersExecuting);

  const templateEditorOpen = useModelProvidersPageStore(selectModelProvidersTemplateEditorOpen);
  const setTemplateEditorOpen = useModelProvidersPageStore(selectSetModelProvidersTemplateEditorOpen);
  const templateLoading = useModelProvidersPageStore(selectModelProvidersTemplateLoading);
  const setTemplateLoading = useModelProvidersPageStore(selectSetModelProvidersTemplateLoading);
  const templateNewItem = useModelProvidersPageStore(selectModelProvidersTemplateNewItem);
  const setTemplateNewItem = useModelProvidersPageStore(selectSetModelProvidersTemplateNewItem);

  const resettingWeights = useModelProvidersPageStore(selectModelProvidersResettingWeights);
  const setResettingWeights = useModelProvidersPageStore(selectSetModelProvidersResettingWeights);
  const resettingPriorities = useModelProvidersPageStore(selectModelProvidersResettingPriorities);
  const setResettingPriorities = useModelProvidersPageStore(selectSetModelProvidersResettingPriorities);
  const enablingAssociations = useModelProvidersPageStore(selectModelProvidersEnablingAssociations);
  const setEnablingAssociations = useModelProvidersPageStore(selectSetModelProvidersEnablingAssociations);

  const batchTesting = useModelProvidersPageStore(selectModelProvidersBatchTesting);
  const setBatchTesting = useModelProvidersPageStore(selectSetModelProvidersBatchTesting);
  const batchTestProgress = useModelProvidersPageStore(selectModelProvidersBatchTestProgress);
  const setBatchTestProgress = useModelProvidersPageStore(selectSetModelProvidersBatchTestProgress);
  const associationTestResults = useModelProvidersPageStore(selectModelProvidersAssociationTestResults);
  const setAssociationTestResults = useModelProvidersPageStore(selectSetModelProvidersAssociationTestResults);

  const blacklistDialogOpen = useModelProvidersPageStore(selectModelProvidersBlacklistDialogOpen);
  const setBlacklistDialogOpen = useModelProvidersPageStore(selectSetModelProvidersBlacklistDialogOpen);
  const blacklistedIds = useModelProvidersPageStore(selectModelProvidersBlacklistedIds);
  const setBlacklistedIds = useModelProvidersPageStore(selectSetModelProvidersBlacklistedIds);
  const blacklistLoading = useModelProvidersPageStore(selectModelProvidersBlacklistLoading);
  const setBlacklistLoading = useModelProvidersPageStore(selectSetModelProvidersBlacklistLoading);
  const blacklistSaving = useModelProvidersPageStore(selectModelProvidersBlacklistSaving);
  const setBlacklistSaving = useModelProvidersPageStore(selectSetModelProvidersBlacklistSaving);
  const blacklistSearchTerm = useModelProvidersPageStore(selectModelProvidersBlacklistSearchTerm);
  const setBlacklistSearchTerm = useModelProvidersPageStore(selectSetModelProvidersBlacklistSearchTerm);
  const blacklistFilter = useModelProvidersPageStore(selectModelProvidersBlacklistFilter);
  const setBlacklistFilter = useModelProvidersPageStore(selectSetModelProvidersBlacklistFilter);

  const resetTransient = useModelProvidersPageStore(selectResetModelProvidersTransient);

  return {
    open,
    setOpen,
    editingAssociation,
    setEditingAssociation,
    deleteId,
    setDeleteId,
    testDialogOpen,
    setTestDialogOpen,
    selectedTestId,
    setSelectedTestId,
    testType,
    setTestType,
    reactTestResult,
    setReactTestResult,
    isSubmitting,
    setIsSubmitting,
    modelListDialogOpen,
    setModelListDialogOpen,
    modelSearchKeyword,
    setModelSearchKeyword,
    selectedProviderModels,
    setSelectedProviderModels,
    selectedAssociationIds,
    setSelectedAssociationIds,
    collapsedProviders,
    setCollapsedProviders,
    batchDeleteDialogOpen,
    setBatchDeleteDialogOpen,
    batchDeleting,
    setBatchDeleting,
    batchUpdatingStatus,
    setBatchUpdatingStatus,
    batchActionSheetOpen,
    setBatchActionSheetOpen,
    batchCapabilitiesDialogOpen,
    setBatchCapabilitiesDialogOpen,
    batchUpdatingCapabilities,
    setBatchUpdatingCapabilities,
    searchKeyword,
    setSearchKeyword,
    selectedProviderType,
    setSelectedProviderType,
    selectedProviderFilter,
    setSelectedProviderFilter,
    selectedStatusFilter,
    setSelectedStatusFilter,
    operationScope,
    setOperationScope,
    filterPanelOpen,
    setFilterPanelOpen,
    previewDialogOpen,
    setPreviewDialogOpen,
    previewType,
    setPreviewType,
    executing,
    setExecuting,
    templateEditorOpen,
    setTemplateEditorOpen,
    templateLoading,
    setTemplateLoading,
    templateNewItem,
    setTemplateNewItem,
    resettingWeights,
    setResettingWeights,
    resettingPriorities,
    setResettingPriorities,
    enablingAssociations,
    setEnablingAssociations,
    batchTesting,
    setBatchTesting,
    batchTestProgress,
    setBatchTestProgress,
    associationTestResults,
    setAssociationTestResults,
    blacklistDialogOpen,
    setBlacklistDialogOpen,
    blacklistedIds,
    setBlacklistedIds,
    blacklistLoading,
    setBlacklistLoading,
    blacklistSaving,
    setBlacklistSaving,
    blacklistSearchTerm,
    setBlacklistSearchTerm,
    blacklistFilter,
    setBlacklistFilter,
    resetTransient,
  };
}
