import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm } from "react-hook-form";
import {
  selectModelProvidersAssociationTestResults,
  selectModelProvidersBatchDeleteDialogOpen,
  selectModelProvidersBatchDeleting,
  selectModelProvidersBatchTestProgress,
  selectModelProvidersBatchTesting,
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
  selectModelProvidersLoading,
  selectModelProvidersLoadingProviderModels,
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
  selectSetModelProvidersBatchDeleteDialogOpen,
  selectSetModelProvidersBatchDeleting,
  selectSetModelProvidersBatchTestProgress,
  selectSetModelProvidersBatchTesting,
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
  selectSetModelProvidersLoading,
  selectSetModelProvidersLoadingProviderModels,
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
import type {
  Model,
  Provider,
  Settings,
} from "@/lib/api";
import { formSchema, type FormValues } from "../form-schema";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";
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
import { useModelProvidersTemplateEditor } from "./use-model-providers-template-editor";
import { useModelProvidersTesting } from "./use-model-providers-testing";

export function useModelProvidersPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const { providerStatus, healthStatus, loadProviderStatus } = useModelProvidersAssociationStatus(models);

  const loading = useModelProvidersPageStore(selectModelProvidersLoading);
  const setLoading = useModelProvidersPageStore(selectSetModelProvidersLoading);
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

  const loadingProviderModels = useModelProvidersPageStore(selectModelProvidersLoadingProviderModels);
  const setLoadingProviderModels = useModelProvidersPageStore(selectSetModelProvidersLoadingProviderModels);
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

  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);

  // 初始化表单
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      model_id: 0,
      provider_name: "",
      provider_id: 0,
      tool_call: true,
      structured_output: false,
      image: false,
      with_header: false,
      weight: 5,
      priority: 10,
      customer_headers: [],
    },
  });
  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: "customer_headers",
  });

  useModelProvidersBootstrap({
    models,
    setModels,
    setProviders,
    setSettings,
    setLoading,
    setLoadingProviderModels,
    setProviderModelGroups,
    setProviderModels,
    setCollapsedProviders,
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
    setLoading,
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

  const shouldShowInitialLoading = loading && models.length === 0 && providers.length === 0;

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

  const operationScopeToolbarProps = {
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
  };

  const associationFilterPanelProps = {
    filterPanelOpen,
    onFilterPanelOpenChange: setFilterPanelOpen,
    activeFilterCount,
    selectedModelId,
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
  };

  const batchTestProgressCardProps = {
    batchTesting,
    batchTestProgress,
    associationTestResults,
    onCancel: handleCancelBatchTest,
    onClear: clearBatchTestResults,
    onSelectSuccess: selectAllSuccessful,
    onSelectFailed: selectAllFailed,
  };

  const associationListSectionProps = {
    loading,
    selectedModelId,
    hasAssociationFilter,
    associations: filteredModelProviders,
    providers,
    selectedAssociationIds,
    isAllSelected: isAllAssociationsSelected,
    isPartialSelected: isPartialAssociationsSelected,
    providerStatus,
    healthStatus,
    statusUpdating,
    associationTestResults,
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
  };

  const blacklistDialogProps = {
    open: blacklistDialogOpen,
    onOpenChange: setBlacklistDialogOpen,
    providers,
    filteredProviders,
    blacklistedIds,
    loading: blacklistLoading,
    saving: blacklistSaving,
    searchTerm: blacklistSearchTerm,
    filter: blacklistFilter,
    onSearchTermChange: setBlacklistSearchTerm,
    onFilterChange: setBlacklistFilter,
    onToggle: handleToggleBlacklist,
    onSave: handleSaveBlacklist,
    onCancel: cancelBlacklistDialog,
  };

  const templateEditorDialogProps = {
    open: templateEditorOpen,
    onOpenChange: setTemplateEditorOpen,
    selectedModelId,
    loading: templateLoading,
    templateData,
    newItem: templateNewItem,
    onNewItemChange: setTemplateNewItem,
    onAdd: addTemplateItem,
    onDelete: deleteTemplateItem,
  };

  const associationFormDialogProps = {
    open,
    onOpenChange: setOpen,
    editingAssociation,
    form,
    models,
    providers,
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
  };

  const testDialogProps = {
    open: testDialogOpen,
    onOpenChange: setTestDialogOpen,
    testType,
    onTestTypeChange: setTestType,
    selectedTestId,
    testResults,
    structuredTestResults,
    reactTestResult,
    onClose: dialogClose,
    onExecute: executeTestNow,
  };

  const modelListDialogProps = {
    open: modelListDialogOpen,
    onOpenChange: setModelListDialogOpen,
    modelSearchKeyword,
    onModelSearchKeywordChange: setModelSearchKeyword,
    loadingProviderModels,
    providerModels,
    visibleProviderGroups,
    visibleAvailableModels,
    visibleExistingCount,
    selectedProviderModels,
    selectedKeys,
    existingAssociationKeys,
    collapsedProviders,
    onToggleProviderCollapse: toggleProviderCollapse,
    onSelectAllVisibleAvailable: selectAllVisibleAvailable,
    onClearSelection: clearModelListSelection,
    onToggleModelSelection: toggleModelSelection,
  };

  const previewDialogProps = {
    open: previewDialogOpen,
    onOpenChange: setPreviewDialogOpen,
    type: previewType,
    data: previewData,
    executing,
    onConfirm: confirmPreviewAction,
  };

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

