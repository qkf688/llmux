import { buildAssociationPayload } from "../utils/payload";
import { useModelProvidersAssociationDialog } from "./use-model-providers-association-dialog";
import { useModelProvidersAssociationMutations } from "./use-model-providers-association-mutations";
import { useModelProvidersAssociationStatusToggle } from "./use-model-providers-association-status-toggle";
import { useModelProvidersAssociationsData } from "./use-model-providers-associations-data";
import { useModelProvidersAssociationFilters } from "./use-model-providers-association-filters";
import { useModelProvidersModelListVisibility } from "./use-model-providers-model-list-visibility";
import { useModelProvidersBatch } from "./use-model-providers-batch";
import { useModelProvidersBlacklist } from "./use-model-providers-blacklist";
import { useModelProvidersBootstrap } from "./use-model-providers-bootstrap";
import { useModelProvidersModelChange } from "./use-model-providers-model-change";
import { useModelProvidersOperationScope } from "./use-model-providers-operation-scope";
import { useModelProvidersAssociationStatus } from "./use-model-providers-association-status";
import { useModelProvidersPageActions } from "./use-model-providers-page-actions";
import { useModelProvidersModelListSelection } from "./use-model-providers-model-list-selection";
import { useModelProvidersPreview } from "./use-model-providers-preview";
import { useModelProvidersPageSectionProps } from "./use-model-providers-page-section-props";
import { useModelProvidersPageDialogProps } from "./use-model-providers-page-dialog-props";
import { useModelProvidersTemplateEditor } from "./use-model-providers-template-editor";
import { useModelProvidersTesting } from "./use-model-providers-testing";
import { useModelProvidersAssociationForm } from "./use-model-providers-association-form";
import { useModelProvidersPageLocalState } from "./use-model-providers-page-local-state";
import { useModelProvidersPageStoreState } from "./use-model-providers-page-store";

export function useModelProvidersPage() {
  const {
    models,
    providers,
    searchParams,
    setSearchParams,
    selectedModelId,
    setSelectedModelId,
    statusUpdating,
    setStatusUpdating,
    statusError,
    setStatusError,
    providerModelGroups,
    providerModels,
    settings,
    loading: dataLoading,
  } = useModelProvidersPageLocalState();

  const { providerStatus, healthStatus, loadProviderStatus } = useModelProvidersAssociationStatus(models);

  const {
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
  } = useModelProvidersPageStoreState();

  const { form, headerFields, appendHeader, removeHeader } = useModelProvidersAssociationForm();

  useModelProvidersBootstrap({
    models,
    resetTransient,
    selectedModelId,
    setSelectedModelId,
    searchParams,
    setSearchParams,
    setFormModelId: (value) => form.setValue("model_id", value),
  });

  const { filteredProviders, openBlacklistDialog, cancelBlacklistDialog, handleSaveBlacklist, handleToggleBlacklist } =
    useModelProvidersBlacklist({
      providers,
      blacklistDialogOpen,
      setBlacklistDialogOpen,
      blacklistedIds,
      setBlacklistedIds,
      setBlacklistLoading,
      setBlacklistSaving,
      blacklistSearchTerm,
      setBlacklistSearchTerm,
      blacklistFilter,
      setBlacklistFilter,
    });

  const { templateData, handleToggleTemplateEditor, handleAddTemplateItem, handleDeleteTemplateItem } =
    useModelProvidersTemplateEditor({
      templateEditorOpen,
      setTemplateEditorOpen,
      setTemplateLoading,
      selectedModelId,
      templateNewItem,
      setTemplateNewItem,
    });

  const buildPayload = buildAssociationPayload;

  const { modelProviders, setModelProviders, fetchModelProviders } = useModelProvidersAssociationsData({
    selectedModelId,
    loadProviderStatus,
  });

  const { handleCreate, handleUpdate, handleDelete } = useModelProvidersAssociationMutations({
    form,
    buildPayload,
    selectedModelId,
    settings,
    fetchModelProviders,
    isSubmitting,
    setIsSubmitting,
    selectedProviderModels,
    setSelectedProviderModels,
    setOpen,
    editingAssociation,
    setEditingAssociation,
    deleteId,
    setDeleteId,
  });

  const { handleStatusToggle } = useModelProvidersAssociationStatusToggle({
    setModelProviders,
    setStatusUpdating,
    setStatusError,
  });

  const { testResults, structuredTestResults, handleTest, dialogClose, executeTestNow } = useModelProvidersTesting({
    setTestDialogOpen,
    setSelectedTestId,
    setTestType,
    setReactTestResult,
    selectedTestId,
    testType,
  });

  const { openEditDialog, openCreateDialog } = useModelProvidersAssociationDialog({
    form,
    settings,
    selectedModelId,
    setOpen,
    setEditingAssociation,
    setSelectedProviderModels,
  });

  const { handleModelChange } = useModelProvidersModelChange({
    searchParams,
    setSearchParams,
    setSelectedModelId,
    clearSelectedAssociationIds: () => setSelectedAssociationIds([]),
    clearSelectedProviderModels: () => setSelectedProviderModels([]),
    closeTemplateEditor: () => setTemplateEditorOpen(false),
    resetAssociationTestResults: () => setAssociationTestResults({}),
    resetSelectedStatusFilter: () => setSelectedStatusFilter("all"),
    form,
  });

  const {
    providerTypes,
    filteredModelProviders,
    hasAssociationFilter,
    activeFilterCount,
    isAllAssociationsSelected,
    isPartialAssociationsSelected,
  } = useModelProvidersAssociationFilters({
    modelProviders,
    providers,
    selectedProviderType,
    selectedProviderFilter,
    selectedStatusFilter,
    searchKeyword,
    selectedAssociationIds,
  });

  const { existingAssociationKeys, visibleProviderGroups, visibleAvailableModels, visibleExistingCount, selectedKeys } =
    useModelProvidersModelListVisibility({
      control: form.control,
      providerModelGroups,
      modelSearchKeyword,
      modelProviders,
      selectedProviderModels,
    });
  const selectedModel = models.find((model) => model.ID === selectedModelId) || null;
  const isGlobalScope = operationScope === "all";

  const shouldShowInitialLoading = dataLoading && models.length === 0 && providers.length === 0;

  const { handleResetWeights, handleResetPriorities, handleEnableAssociations } = useModelProvidersOperationScope({
    selectedModelId,
    isGlobalScope,
    fetchModelProviders,
    setResettingWeights,
    setResettingPriorities,
    setEnablingAssociations,
  });

  const { previewData, handleAutoAssociate, handleCleanInvalid, executePreviewAction } = useModelProvidersPreview({
    selectedModelId,
    previewType,
    setPreviewType,
    setPreviewDialogOpen,
    setExecuting,
    fetchModelProviders,
  });

  const {
    handleSelectAllAssociations,
    handleSelectOneAssociation,
    handleBatchDeleteAssociations,
    handleBatchUpdateStatus,
    handleBatchUpdateCapabilities,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    selectAllSuccessful,
    selectAllFailed,
    clearBatchTestResults,
  } = useModelProvidersBatch({
    selectedModelId,
    fetchModelProviders,
    filteredModelProviders,
    selectedAssociationIds,
    setSelectedAssociationIds,
    setModelProviders,
    setBatchDeleteDialogOpen,
    setBatchDeleting,
    setBatchUpdatingStatus,
    setBatchCapabilitiesDialogOpen,
    setBatchUpdatingCapabilities,
    setBatchTesting,
    setBatchTestProgress,
    associationTestResults,
    setAssociationTestResults,
  });

  const {
    openModelListDialog,
    clearSelectedProviderModels,
    removeSelectedProviderModel,
    handleProviderChange,
    selectAllVisibleAvailable,
    clearModelListSelection,
    toggleModelSelection,
  } = useModelProvidersModelListSelection({
    setModelSearchKeyword,
    setModelListDialogOpen,
    setSelectedProviderModels,
    visibleAvailableModels,
  });

  const {
    openDeleteDialog,
    toggleProviderCollapse,
    refreshStatus,
    handleDeleteDialogChange,
    confirmPreviewAction,
    addTemplateItem,
    deleteTemplateItem,
  } = useModelProvidersPageActions({
    selectedModelId,
    modelProviders,
    setDeleteId,
    setCollapsedProviders,
    loadProviderStatus,
    executePreviewAction,
    handleAddTemplateItem,
    handleDeleteTemplateItem,
  });

  const { operationScopeToolbarProps, associationFilterPanelProps, batchTestProgressCardProps, associationListSectionProps } =
    useModelProvidersPageSectionProps({
      operationScope,
      onOperationScopeChange: setOperationScope,
      selectedModelName: isGlobalScope ? "全部" : (selectedModel?.Name ?? "未选择"),
      selectedModelId,
      resettingWeights,
      resettingPriorities,
      enablingAssociations,
      onResetWeights: handleResetWeights,
      onResetPriorities: handleResetPriorities,
      onEnableAssociations: handleEnableAssociations,

      filterPanelOpen,
      onFilterPanelOpenChange: setFilterPanelOpen,
      activeFilterCount,
      models,
      onModelChange: handleModelChange,
      selectedProviderType,
      onSelectedProviderTypeChange: setSelectedProviderType,
      selectedProviderFilter,
      onSelectedProviderFilterChange: setSelectedProviderFilter,
      selectedStatusFilter,
      onSelectedStatusFilterChange: setSelectedStatusFilter,
      searchKeyword,
      onSearchKeywordChange: setSearchKeyword,
      providers,
      providerTypes,
      selectedAssociationCount: selectedAssociationIds.length,
      batchUpdatingStatus,
      batchActionSheetOpen,
      onBatchActionSheetOpenChange: setBatchActionSheetOpen,
      batchCapabilitiesDialogOpen,
      onBatchCapabilitiesDialogOpenChange: setBatchCapabilitiesDialogOpen,
      batchUpdatingCapabilities,
      onBatchUpdateCapabilities: handleBatchUpdateCapabilities,
      batchTesting,
      filteredAssociationCount: filteredModelProviders.length,
      associationTestResults,
      batchDeleteDialogOpen,
      onBatchDeleteDialogOpenChange: setBatchDeleteDialogOpen,
      batchDeleting,
      onBatchDeleteConfirm: handleBatchDeleteAssociations,
      onBatchUpdateStatus: handleBatchUpdateStatus,
      onBatchTestSelected: handleBatchTestSelected,
      onBatchTestAll: handleBatchTestAll,
      onSelectAllSuccessful: selectAllSuccessful,
      onSelectAllFailed: selectAllFailed,
      onToggleTemplateEditor: handleToggleTemplateEditor,
      onOpenBlacklistDialog: openBlacklistDialog,
      onAutoAssociate: handleAutoAssociate,
      onCleanInvalid: handleCleanInvalid,
      onOpenCreateDialog: openCreateDialog,

      batchTestProgress,
      onCancelBatchTest: handleCancelBatchTest,
      onClearBatchTestResults: clearBatchTestResults,

      loading: dataLoading,
      hasAssociationFilter,
      associations: filteredModelProviders,
      selectedAssociationIds,
      isAllSelected: isAllAssociationsSelected,
      isPartialSelected: isPartialAssociationsSelected,
      providerStatus,
      healthStatus,
      statusUpdating,
      deleteId,
      onSelectAll: handleSelectAllAssociations,
      onSelectOne: handleSelectOneAssociation,
      onRefreshStatus: refreshStatus,
      onToggleStatus: handleStatusToggle,
      onEdit: openEditDialog,
      onOpenDelete: openDeleteDialog,
      onDeleteDialogChange: handleDeleteDialogChange,
      onDeleteConfirm: handleDelete,
      onTest: handleTest,
    });

  const {
    blacklistDialogProps,
    templateEditorDialogProps,
    associationFormDialogProps,
    testDialogProps,
    modelListDialogProps,
    previewDialogProps,
  } = useModelProvidersPageDialogProps({
    blacklistDialogOpen,
    onBlacklistDialogOpenChange: setBlacklistDialogOpen,
    providers,
    filteredProviders,
    blacklistedIds,
    blacklistLoading,
    blacklistSaving,
    blacklistSearchTerm,
    blacklistFilter,
    onBlacklistSearchTermChange: setBlacklistSearchTerm,
    onBlacklistFilterChange: setBlacklistFilter,
    onToggleBlacklist: handleToggleBlacklist,
    onSaveBlacklist: handleSaveBlacklist,
    onCancelBlacklist: cancelBlacklistDialog,

    templateEditorOpen,
    onTemplateEditorOpenChange: setTemplateEditorOpen,
    selectedModelId,
    templateLoading,
    templateData,
    templateNewItem,
    onTemplateNewItemChange: setTemplateNewItem,
    onAddTemplateItem: addTemplateItem,
    onDeleteTemplateItem: deleteTemplateItem,

    associationFormOpen: open,
    onAssociationFormOpenChange: setOpen,
    editingAssociation,
    form,
    models,
    providersForForm: providers,
    selectedProviderModels,
    isSubmitting,
    headerFields,
    appendHeader,
    removeHeader,
    onSubmitCreate: handleCreate,
    onSubmitUpdate: handleUpdate,
    onOpenModelListDialog: openModelListDialog,
    onClearSelectedProviderModels: clearSelectedProviderModels,
    onRemoveSelectedProviderModel: removeSelectedProviderModel,
    onProviderChange: handleProviderChange,

    testDialogOpen,
    onTestDialogOpenChange: setTestDialogOpen,
    testType,
    onTestTypeChange: setTestType,
    selectedTestId,
    testResults,
    structuredTestResults,
    reactTestResult,
    onCloseTestDialog: dialogClose,
    onExecuteTestNow: executeTestNow,

    modelListDialogOpen,
    onModelListDialogOpenChange: setModelListDialogOpen,
    modelSearchKeyword,
    onModelSearchKeywordChange: setModelSearchKeyword,
    loadingProviderModels: dataLoading,
    providerModels,
    visibleProviderGroups,
    visibleAvailableModels,
    visibleExistingCount,
    selectedProviderModelsForList: selectedProviderModels,
    selectedKeys,
    existingAssociationKeys,
    collapsedProviders,
    onToggleProviderCollapse: toggleProviderCollapse,
    onSelectAllVisibleAvailable: selectAllVisibleAvailable,
    onClearModelListSelection: clearModelListSelection,
    onToggleModelSelection: toggleModelSelection,

    previewDialogOpen,
    onPreviewDialogOpenChange: setPreviewDialogOpen,
    previewType,
    previewData,
    executing,
    onConfirmPreview: confirmPreviewAction,
  });

  return {
    shouldShowInitialLoading,
    statusError,
    operationScopeToolbarProps,
    associationFilterPanelProps,
    batchTestProgressCardProps,
    associationListSectionProps,
    blacklistDialogProps,
    templateEditorDialogProps,
    associationFormDialogProps,
    testDialogProps,
    modelListDialogProps,
    previewDialogProps,
  };
}

