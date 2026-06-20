import { useEffect, useMemo } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { defaultProviderFormValues, providerFormSchema, type ProviderFormValues } from "./form-schema";
import {
  selectAddingModels,
  selectAllModelsOpen,
  selectAllModelsProvider,
  selectAllModelsSearchQuery,
  selectAllModelsTestResults,
  selectAllModelsTypeFilter,
  selectAvailableTypes,
  selectBatchTestProgress,
  selectBatchTesting,
  selectClearAssociationId,
  selectClearingAssociation,
  selectCustomModelInput,
  selectDebouncedNameFilter,
  selectDeleteId,
  selectEditingProvider,
  selectFlushNameFilter,
  selectModelsLoading,
  selectModelsOpen,
  selectModelsOpenId,
  selectNameFilter,
  selectProviderDialogOpen,
  selectResetProvidersTransient,
  selectSelectedAllModels,
  selectSelectedUpstreamModels,
  selectSetAddingModels,
  selectSetAllModelsOpen,
  selectSetAllModelsProvider,
  selectSetAllModelsSearchQuery,
  selectSetAllModelsTestResults,
  selectSetAllModelsTypeFilter,
  selectSetAvailableTypes,
  selectSetBatchTestProgress,
  selectSetBatchTesting,
  selectSetClearAssociationId,
  selectSetClearingAssociation,
  selectSetCustomModelInput,
  selectSetDebouncedNameFilter,
  selectSetDeleteId,
  selectSetEditingProvider,
  selectSetModelsLoading,
  selectSetModelsOpen,
  selectSetModelsOpenId,
  selectSetNameFilter,
  selectSetProviderDialogOpen,
  selectSetProviders,
  selectSetSelectedAllModels,
  selectSetSelectedUpstreamModels,
  selectSetShowApiKey,
  selectSetSyncingAll,
  selectSetSyncingModels,
  selectSetTypeFilter,
  selectSetUpstreamBatchTesting,
  selectSetUpstreamBatchTestProgress,
  selectSetUpstreamTestResults,
  selectShowApiKey,
  selectSyncingAll,
  selectSyncingModels,
  selectToggleShowApiKey,
  selectTypeFilter,
  selectUpstreamBatchTesting,
  selectUpstreamBatchTestProgress,
  selectUpstreamTestResults,
  useProvidersPageStore,
} from "@/stores/providers";
import { useProviderModelTesting } from "./hooks/use-provider-model-testing";
import { useAllModelsDialog } from "./hooks/use-all-models-dialog";
import { useUpstreamModelsDialog } from "./hooks/use-upstream-models-dialog";
import { useProviderDialog } from "./hooks/use-provider-dialog";
import { useProviderDangerActions } from "./hooks/use-provider-danger-actions";
import { useProviderMutations } from "./hooks/use-provider-mutations";
import { useProviderSyncActions } from "./hooks/use-provider-sync-actions";
import { useProviderSwitchActions } from "./hooks/use-provider-switch-actions";
import { useProviders, useProviderTemplates, useSettings } from "@/hooks/api/use-providers";
import { getAllModelsForProvider } from "./utils/provider-models";
import { hasActiveProvidersFilter } from "./utils/filters";
import { ProvidersListSection } from "./components/sections/providers-list-section";
import { ProvidersToolbar } from "./components/sections/providers-toolbar";
import { ProviderFormDialog } from "./components/dialogs/provider-form-dialog";
import { AllModelsDialog } from "./components/dialogs/all-models-dialog";
import { UpstreamModelsDialog } from "./components/dialogs/upstream-models-dialog";

export default function ProvidersPage() {
  const setProviders = useProvidersPageStore(selectSetProviders);
  const setAvailableTypes = useProvidersPageStore(selectSetAvailableTypes);

  const nameFilter = useProvidersPageStore(selectNameFilter);
  const setNameFilter = useProvidersPageStore(selectSetNameFilter);
  const debouncedNameFilter = useProvidersPageStore(selectDebouncedNameFilter);
  const setDebouncedNameFilter = useProvidersPageStore(selectSetDebouncedNameFilter);
  const typeFilter = useProvidersPageStore(selectTypeFilter);
  const setTypeFilter = useProvidersPageStore(selectSetTypeFilter);
  const availableTypes = useProvidersPageStore(selectAvailableTypes);
  const flushNameFilter = useProvidersPageStore(selectFlushNameFilter);

  const filters = useMemo(
    () => ({
      name: debouncedNameFilter.trim() || undefined,
      type: typeFilter === "all" ? undefined : typeFilter,
    }),
    [debouncedNameFilter, typeFilter],
  );

  const { data: providers = [], isLoading: loading } = useProviders(filters);
  const { data: providerTemplates = [] } = useProviderTemplates();
  const { data: settings } = useSettings();

  const autoAssociateOnAddEnabled = settings?.auto_associate_on_add ?? false;
  const autoCleanOnDeleteEnabled = settings?.auto_clean_on_delete ?? false;

  useEffect(() => {
    setProviders(providers);
  }, [providers, setProviders]);

  useEffect(() => {
    if (providerTemplates.length > 0) {
      setAvailableTypes(providerTemplates.map((t) => t.type));
    }
  }, [providerTemplates, setAvailableTypes]);

  const clearingAssociation = useProvidersPageStore(selectClearingAssociation);
  const setClearingAssociation = useProvidersPageStore(selectSetClearingAssociation);
  const modelsLoading = useProvidersPageStore(selectModelsLoading);
  const setModelsLoading = useProvidersPageStore(selectSetModelsLoading);
  const addingModels = useProvidersPageStore(selectAddingModels);
  const setAddingModels = useProvidersPageStore(selectSetAddingModels);
  const syncingModels = useProvidersPageStore(selectSyncingModels);
  const setSyncingModels = useProvidersPageStore(selectSetSyncingModels);
  const syncingAll = useProvidersPageStore(selectSyncingAll);
  const setSyncingAll = useProvidersPageStore(selectSetSyncingAll);

  const open = useProvidersPageStore(selectProviderDialogOpen);
  const setOpen = useProvidersPageStore(selectSetProviderDialogOpen);
  const editingProvider = useProvidersPageStore(selectEditingProvider);
  const setEditingProvider = useProvidersPageStore(selectSetEditingProvider);
  const deleteId = useProvidersPageStore(selectDeleteId);
  const setDeleteId = useProvidersPageStore(selectSetDeleteId);
  const clearAssociationId = useProvidersPageStore(selectClearAssociationId);
  const setClearAssociationId = useProvidersPageStore(selectSetClearAssociationId);

  const modelsOpen = useProvidersPageStore(selectModelsOpen);
  const setModelsOpen = useProvidersPageStore(selectSetModelsOpen);
  const modelsOpenId = useProvidersPageStore(selectModelsOpenId);
  const setModelsOpenId = useProvidersPageStore(selectSetModelsOpenId);

  const selectedUpstreamModels = useProvidersPageStore(selectSelectedUpstreamModels);
  const setSelectedUpstreamModels = useProvidersPageStore(selectSetSelectedUpstreamModels);

  const allModelsOpen = useProvidersPageStore(selectAllModelsOpen);
  const setAllModelsOpen = useProvidersPageStore(selectSetAllModelsOpen);
  const allModelsProvider = useProvidersPageStore(selectAllModelsProvider);
  const setAllModelsProvider = useProvidersPageStore(selectSetAllModelsProvider);

  const selectedAllModels = useProvidersPageStore(selectSelectedAllModels);
  const setSelectedAllModels = useProvidersPageStore(selectSetSelectedAllModels);
  const customModelInput = useProvidersPageStore(selectCustomModelInput);
  const setCustomModelInput = useProvidersPageStore(selectSetCustomModelInput);
  const allModelsSearchQuery = useProvidersPageStore(selectAllModelsSearchQuery);
  const setAllModelsSearchQuery = useProvidersPageStore(selectSetAllModelsSearchQuery);

  const allModelsTypeFilter = useProvidersPageStore(selectAllModelsTypeFilter);
  const setAllModelsTypeFilter = useProvidersPageStore(selectSetAllModelsTypeFilter);

  const allModelsTestResults = useProvidersPageStore(selectAllModelsTestResults);
  const setAllModelsTestResults = useProvidersPageStore(selectSetAllModelsTestResults);
  const batchTesting = useProvidersPageStore(selectBatchTesting);
  const setBatchTesting = useProvidersPageStore(selectSetBatchTesting);
  const batchTestProgress = useProvidersPageStore(selectBatchTestProgress);
  const setBatchTestProgress = useProvidersPageStore(selectSetBatchTestProgress);

  const upstreamTestResults = useProvidersPageStore(selectUpstreamTestResults);
  const setUpstreamTestResults = useProvidersPageStore(selectSetUpstreamTestResults);
  const upstreamBatchTesting = useProvidersPageStore(selectUpstreamBatchTesting);
  const setUpstreamBatchTesting = useProvidersPageStore(selectSetUpstreamBatchTesting);
  const upstreamBatchTestProgress = useProvidersPageStore(selectUpstreamBatchTestProgress);
  const setUpstreamBatchTestProgress = useProvidersPageStore(selectSetUpstreamBatchTestProgress);

  const showApiKey = useProvidersPageStore(selectShowApiKey);
  const setShowApiKey = useProvidersPageStore(selectSetShowApiKey);
  const toggleShowApiKey = useProvidersPageStore(selectToggleShowApiKey);

  const resetTransient = useProvidersPageStore(selectResetProvidersTransient);

  // 筛选条件
  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  // 初始化表单
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: { ...defaultProviderFormValues },
  });

  // 监听类型变化，用于显示/隐藏 Anthropic 特有字段
  const watchedType = form.watch("type");

  useEffect(() => {
    const timeoutId = window.setTimeout(() => {
      setDebouncedNameFilter(nameFilter);
    }, 250);
    return () => window.clearTimeout(timeoutId);
  }, [nameFilter, setDebouncedNameFilter]);

  const autoActionsFlags = { autoAssociateOnAddEnabled, autoCleanOnDeleteEnabled };

  const {
    allModelsList,
    filteredAllModels,
    upstreamSet,
    isAllFilteredSelected,
    toggleSelectAllModels,
    setAllModelsList,
    upstreamModelsList,
    upstreamStatus,
    openAllModelsDialog,
    persistModels,
    handleAddCustomModels,
    handleRemoveModelFromAll,
    handleRemoveSelectedModels,
    handleSyncUpstreamModels,
  } = useAllModelsDialog({
    setProviders,
    allModelsProvider,
    setAllModelsProvider,
    setAllModelsOpen,
    selectedAllModels,
    setSelectedAllModels,
    customModelInput,
    setCustomModelInput,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
    allModelsTypeFilter,
    setAllModelsTypeFilter,
    setAllModelsTestResults,
    setAddingModels,
    setSyncingModels,
    autoActionsFlags,
  });

  const {
    providerModels,
    filteredProviderModels,
    savedModelSet,
    selectableModelIds,
    isAllSelectableChecked,
    toggleSelectAll,
    openModelsDialog,
    refreshUpstreamModels,
    handleUpstreamSearchChange,
    handleAddUpstreamToAll,
  } = useUpstreamModelsDialog({
    providers,
    modelsOpenId,
    setModelsOpen,
    setModelsOpenId,
    modelsLoading,
    setModelsLoading,
    addingModels,
    setAddingModels,
    selectedUpstreamModels,
    setSelectedUpstreamModels,
    allModelsProvider,
    setAllModelsProvider,
    setAllModelsList,
    persistModels,
    autoActionsFlags,
  });

  const {
    updatingFilter,
    updatingAssociationTrigger,
    handleToggleModelEndpoint,
    handleToggleModelFilter,
    handleToggleAssociationTrigger,
  } = useProviderSwitchActions({ setProviders });


  const {
    copyModelName,
    handleTestAllModel,
    selectAllSuccessful,
    selectAllFailed,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    handleTestUpstreamModel,
    handleBatchTestUpstreamAll,
    handleBatchTestUpstreamSelected,
    handleCancelUpstreamBatchTest,
    selectUpstreamSuccessful,
    selectUpstreamFailed,
  } = useProviderModelTesting({
    allModelsProvider,
    modelsOpenId,
    filteredAllModels,
    filteredProviderModels,
    selectedAllModels,
    selectedUpstreamModels,
    allModelsTestResults,
    upstreamTestResults,
    setSelectedAllModels,
    setSelectedUpstreamModels,
    setAllModelsTestResults,
    setUpstreamTestResults,
    setBatchTesting,
    setBatchTestProgress,
    setUpstreamBatchTesting,
    setUpstreamBatchTestProgress,
  });

  const { handleSyncAllProviders } = useProviderSyncActions({ setSyncingAll });

  const { handleSubmitProvider } = useProviderMutations({
    form,
    editingProvider,
    setEditingProvider,
    setOpen,
  });

  const { openEditDialog, openCreateDialog } = useProviderDialog({
    form,
    setOpen,
    setEditingProvider,
    setShowApiKey,
  });

  const {
    openDeleteDialog,
    cancelDeleteDialog,
    handleDelete,
    openClearAssociationsDialog,
    cancelClearAssociationsDialog,
    handleClearAssociations,
  } = useProviderDangerActions({
    providers,
    deleteId,
    setDeleteId,
    clearAssociationId,
    setClearAssociationId,
    clearingAssociation,
    setClearingAssociation,
    autoCleanOnDeleteEnabled,
  });

  const hasFilter = hasActiveProvidersFilter(nameFilter, typeFilter);

  const providerFormDialogProps = {
    open,
    onOpenChange: setOpen,
    editingProvider,
    form,
    providerTemplates,
    watchedType,
    showApiKey,
    toggleShowApiKey,
    onSubmit: handleSubmitProvider,
  };

  const allModelsDialogProps = {
    open: allModelsOpen,
    onOpenChange: setAllModelsOpen,
    allModelsProvider,
    upstreamStatus,
    upstreamModelsList,
    upstreamSet,
    allModelsList,
    filteredAllModels,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
    allModelsTypeFilter,
    setAllModelsTypeFilter,
    allModelsTestResults,
    batchTesting,
    batchTestProgress,
    syncingModels,
    addingModels,
    selectedAllModels,
    setSelectedAllModels,
    isAllFilteredSelected,
    toggleSelectAllModels,
    handleSyncUpstreamModels,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    selectAllSuccessful,
    selectAllFailed,
    handleRemoveSelectedModels,
    handleTestAllModel,
    copyModelName,
    handleRemoveModelFromAll,
    customModelInput,
    setCustomModelInput,
    handleAddCustomModels,
  };

  const upstreamProviderName = providers.find((v) => v.ID === modelsOpenId)?.Name;
  const cachedModelsCount = getAllModelsForProvider(providers, modelsOpenId || 0).length;

  const upstreamModelsDialogProps = {
    open: modelsOpen,
    onOpenChange: setModelsOpen,
    providerName: upstreamProviderName,
    modelsOpenId,
    modelsLoading,
    addingModels,
    providerModels,
    filteredProviderModels,
    cachedModelsCount,
    savedModelSet,
    selectedUpstreamModels,
    setSelectedUpstreamModels,
    upstreamTestResults,
    upstreamBatchTesting,
    upstreamBatchTestProgress,
    selectableModelIds,
    isAllSelectableChecked,
    toggleSelectAll,
    handleBatchTestUpstreamAll,
    handleBatchTestUpstreamSelected,
    handleCancelUpstreamBatchTest,
    selectUpstreamSuccessful,
    selectUpstreamFailed,
    refreshUpstreamModels,
    handleUpstreamSearchChange,
    handleAddUpstreamToAll,
    handleTestUpstreamModel,
    copyModelName,
  };

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <ProvidersToolbar
        syncingAll={syncingAll}
        onSyncAllProviders={handleSyncAllProviders}
        nameFilter={nameFilter}
        setNameFilter={setNameFilter}
        typeFilter={typeFilter}
        setTypeFilter={setTypeFilter}
        availableTypes={availableTypes}
        flushNameFilter={flushNameFilter}
        onCreateProvider={openCreateDialog}
      />
      <ProvidersListSection
        loading={loading}
        hasFilter={hasFilter}
        providers={providers}
        updatingFilter={updatingFilter}
        updatingAssociationTrigger={updatingAssociationTrigger}
        clearingAssociation={clearingAssociation}
        onOpenAllModelsDialog={openAllModelsDialog}
        onToggleModelEndpoint={handleToggleModelEndpoint}
        onToggleAssociationTrigger={handleToggleAssociationTrigger}
        onToggleModelFilter={handleToggleModelFilter}
        onEditProvider={openEditDialog}
        onOpenModelsDialog={openModelsDialog}
        onOpenClearAssociationsDialog={openClearAssociationsDialog}
        onCancelClearAssociationsDialog={cancelClearAssociationsDialog}
        onHandleClearAssociations={handleClearAssociations}
        onOpenDeleteDialog={openDeleteDialog}
        onCancelDeleteDialog={cancelDeleteDialog}
        onHandleDelete={handleDelete}
      />

      <ProviderFormDialog {...providerFormDialogProps} />

      <AllModelsDialog {...allModelsDialogProps} />

      <UpstreamModelsDialog {...upstreamModelsDialogProps} />
    </div>
  );
}
 
