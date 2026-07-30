import { useCallback } from "react";
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
import { useModelProvidersOperationScope } from "./use-model-providers-operation-scope";
import { useModelProvidersAssociationStatus } from "./use-model-providers-association-status";
import { useModelProvidersModelListSelection } from "./use-model-providers-model-list-selection";
import { useModelProvidersPreview } from "./use-model-providers-preview";
import { useModelProvidersPageSectionProps } from "./use-model-providers-page-section-props";
import { useModelProvidersPageDialogProps } from "./use-model-providers-page-dialog-props";
import { useModelProvidersTemplateEditor } from "./use-model-providers-template-editor";
import { useModelProvidersTesting } from "./use-model-providers-testing";
import { useModelProvidersAssociationForm } from "./use-model-providers-association-form";
import { useModelProvidersPageLocalState } from "./use-model-providers-page-local-state";
import { useModelProvidersPageStoreState } from "./use-model-providers-page-store";

/**
 * Model providers 页面 composition root：按依赖顺序调用领域 hook，
 * 组成上下文对象交装配器，返回 10 组 props bag。不保留扁平字段透传。
 */
export function useModelProvidersPage() {
  // 派生数据
  const localState = useModelProvidersPageLocalState();
  const { models, providers, searchParams, setSearchParams, selectedModelId, setSelectedModelId, setStatusUpdating, statusError, setStatusError, providerModelGroups, loading: dataLoading } = localState;

  const store = useModelProvidersPageStoreState();
  const { resetTransient, selectedAssociationIds, setSelectedAssociationIds, selectedProviderModels, setSelectedProviderModels, setCollapsedProviders, setTemplateEditorOpen, setAssociationTestResults, setSelectedStatusFilter, deleteId, setDeleteId, operationScope, isSubmitting, setIsSubmitting, editingAssociation, setEditingAssociation, setOpen, associationTestResults } = store;

  const associationStatus = useModelProvidersAssociationStatus(models);
  const { loadProviderStatus } = associationStatus;

  const form = useModelProvidersAssociationForm();

  useModelProvidersBootstrap({
    models,
    resetTransient,
    selectedModelId,
    setSelectedModelId,
    searchParams,
    setSearchParams,
    setFormModelId: (value) => form.form.setValue("model_id", value),
  });

  const blacklist = useModelProvidersBlacklist({
    providers,
    blacklistDialogOpen: store.blacklistDialogOpen,
    setBlacklistDialogOpen: store.setBlacklistDialogOpen,
    blacklistedIds: store.blacklistedIds,
    setBlacklistedIds: store.setBlacklistedIds,
    setBlacklistLoading: store.setBlacklistLoading,
    setBlacklistSaving: store.setBlacklistSaving,
    blacklistSearchTerm: store.blacklistSearchTerm,
    setBlacklistSearchTerm: store.setBlacklistSearchTerm,
    blacklistFilter: store.blacklistFilter,
    setBlacklistFilter: store.setBlacklistFilter,
  });

  const templateEditor = useModelProvidersTemplateEditor({
    templateEditorOpen: store.templateEditorOpen,
    setTemplateEditorOpen: store.setTemplateEditorOpen,
    setTemplateLoading: store.setTemplateLoading,
    selectedModelId,
    templateNewItem: store.templateNewItem,
    setTemplateNewItem: store.setTemplateNewItem,
  });

  const buildPayload = buildAssociationPayload;

  const associationsData = useModelProvidersAssociationsData({
    selectedModelId,
    loadProviderStatus,
  });

  const mutations = useModelProvidersAssociationMutations({
    form: form.form,
    buildPayload,
    selectedModelId,
    settings: localState.settings,
    fetchModelProviders: associationsData.fetchModelProviders,
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

  const statusToggle = useModelProvidersAssociationStatusToggle({
    setModelProviders: associationsData.setModelProviders,
    setStatusUpdating,
    setStatusError,
  });

  const testing = useModelProvidersTesting({
    setTestDialogOpen: store.setTestDialogOpen,
    setSelectedTestId: store.setSelectedTestId,
    setTestType: store.setTestType,
    setReactTestResult: store.setReactTestResult,
    selectedTestId: store.selectedTestId,
    testType: store.testType,
  });

  const associationDialog = useModelProvidersAssociationDialog({
    form: form.form,
    settings: localState.settings,
    selectedModelId,
    setOpen,
    setEditingAssociation,
    setSelectedProviderModels,
  });

  // 内联：原 useModelProvidersModelChange（单个 useCallback，无独立 state）
  const handleModelChange = useCallback(
    (modelId: string) => {
      const id = parseInt(modelId);
      setSelectedModelId(id);
      setSelectedAssociationIds([]); // 切换模型时清空选择
      setSelectedProviderModels([]);
      setTemplateEditorOpen(false);
      setAssociationTestResults({}); // 切换模型时清空测试结果
      setSelectedStatusFilter("all"); // 切换模型时重置启用状态筛选器
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set("modelId", id.toString());
      setSearchParams(nextParams);
      form.form.setValue("model_id", id);
    },
    [form.form, searchParams, setSearchParams, setSelectedModelId, setSelectedAssociationIds, setSelectedProviderModels, setTemplateEditorOpen, setAssociationTestResults, setSelectedStatusFilter]
  );

  const filters = useModelProvidersAssociationFilters({
    modelProviders: associationsData.modelProviders,
    providers,
    selectedProviderType: store.selectedProviderType,
    selectedProviderFilter: store.selectedProviderFilter,
    selectedStatusFilter: store.selectedStatusFilter,
    searchKeyword: store.searchKeyword,
    selectedAssociationIds,
  });

  const modelListVisibility = useModelProvidersModelListVisibility({
    control: form.form.control,
    providerModelGroups,
    modelSearchKeyword: store.modelSearchKeyword,
    modelProviders: associationsData.modelProviders,
    selectedProviderModels,
  });

  const selectedModel = models.find((model) => model.ID === selectedModelId) || null;
  const isGlobalScope = operationScope === "all";
  const shouldShowInitialLoading = dataLoading && models.length === 0 && providers.length === 0;

  const operationScopeHook = useModelProvidersOperationScope({
    selectedModelId,
    isGlobalScope,
    fetchModelProviders: associationsData.fetchModelProviders,
    setResettingWeights: store.setResettingWeights,
    setResettingPriorities: store.setResettingPriorities,
    setEnablingAssociations: store.setEnablingAssociations,
  });

  const preview = useModelProvidersPreview({
    selectedModelId,
    previewType: store.previewType,
    setPreviewType: store.setPreviewType,
    setPreviewDialogOpen: store.setPreviewDialogOpen,
    setExecuting: store.setExecuting,
    fetchModelProviders: associationsData.fetchModelProviders,
  });

  const batch = useModelProvidersBatch({
    selectedModelId,
    fetchModelProviders: associationsData.fetchModelProviders,
    filteredModelProviders: filters.filteredModelProviders,
    selectedAssociationIds,
    setSelectedAssociationIds,
    setModelProviders: associationsData.setModelProviders,
    setBatchDeleteDialogOpen: store.setBatchDeleteDialogOpen,
    setBatchDeleting: store.setBatchDeleting,
    setBatchUpdatingStatus: store.setBatchUpdatingStatus,
    setBatchCapabilitiesDialogOpen: store.setBatchCapabilitiesDialogOpen,
    setBatchUpdatingCapabilities: store.setBatchUpdatingCapabilities,
    setBatchTesting: store.setBatchTesting,
    setBatchTestProgress: store.setBatchTestProgress,
    associationTestResults,
    setAssociationTestResults,
  });

  const modelListSelection = useModelProvidersModelListSelection({
    setModelSearchKeyword: store.setModelSearchKeyword,
    setModelListDialogOpen: store.setModelListDialogOpen,
    setSelectedProviderModels,
    visibleAvailableModels: modelListVisibility.visibleAvailableModels,
  });

  // 内联：原 useModelProvidersPageActions（纯转发，无 state/effect）
  const openDeleteDialog = (id: number) => setDeleteId(id);
  const toggleProviderCollapse = (providerId: number) =>
    setCollapsedProviders((prev) => ({ ...prev, [providerId]: !prev[providerId] }));
  const refreshStatus = () => {
    if (selectedModelId) {
      void loadProviderStatus(associationsData.modelProviders, selectedModelId);
    }
  };
  const handleDeleteDialogChange = (openValue: boolean) => {
    if (!openValue) {
      setDeleteId(null);
    }
  };
  const confirmPreviewAction = () => {
    void preview.executePreviewAction();
  };
  const addTemplateItem = () => {
    void templateEditor.handleAddTemplateItem();
  };
  const deleteTemplateItem = (name: string) => {
    void templateEditor.handleDeleteTemplateItem(name);
  };

  const ctx = {
    localState,
    store,
    associationStatus,
    filters,
    modelListVisibility,
    form,
    blacklist,
    templateEditor,
    mutations,
    statusToggle,
    operationScope: operationScopeHook,
    preview,
    modelChange: { handleModelChange },
    testing,
    batch,
    pageActions: { openDeleteDialog, toggleProviderCollapse, refreshStatus, handleDeleteDialogChange, confirmPreviewAction, addTemplateItem, deleteTemplateItem },
    associationDialog,
    modelListSelection,
    selectedModel,
    isGlobalScope,
    shouldShowInitialLoading,
  };

  const sections = useModelProvidersPageSectionProps(ctx);
  const dialogs = useModelProvidersPageDialogProps(ctx);

  return {
    shouldShowInitialLoading,
    statusError,
    ...sections,
    ...dialogs,
  };
}
