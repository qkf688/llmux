import { useEffect } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { defaultProviderFormValues, providerFormSchema, type ProviderFormValues } from "./form-schema";
import { useProvidersPageStore } from "@/stores/providers";
import { useProviderModelTesting } from "./hooks/use-provider-model-testing";
import { useAllModelsDialog } from "./hooks/use-all-models-dialog";
import { useUpstreamModelsDialog } from "./hooks/use-upstream-models-dialog";
import { useProviderDialog } from "./hooks/use-provider-dialog";
import { useProviderDangerActions } from "./hooks/use-provider-danger-actions";
import { useProviderMutations } from "./hooks/use-provider-mutations";
import { useProviderSyncActions } from "./hooks/use-provider-sync-actions";
import { useProviderSwitchActions } from "./hooks/use-provider-switch-actions";
import { useProvidersBootstrap } from "./hooks/use-providers-bootstrap";
import { getAllModelsForProvider } from "./utils/provider-models";
import { hasActiveProvidersFilter } from "./utils/filters";
import { ProvidersListSection } from "./components/sections/providers-list-section";
import { ProvidersToolbar } from "./components/sections/providers-toolbar";
import { ProviderFormDialog } from "./components/dialogs/provider-form-dialog";
import { AllModelsDialog } from "./components/dialogs/all-models-dialog";
import { UpstreamModelsDialog } from "./components/dialogs/upstream-models-dialog";

export default function ProvidersPage() {
  // 上游模型测试相关状态
  const {
    loading,
    setLoading,
    providers,
    setProviders,
    providerTemplates,
    setProviderTemplates,
    clearingAssociation,
    setClearingAssociation,
    modelsLoading,
    setModelsLoading,
    addingModels,
    setAddingModels,
    syncingModels,
    setSyncingModels,
    syncingAll,
    setSyncingAll,
    autoAssociateOnAddEnabled,
    setAutoAssociateOnAddEnabled,
    autoCleanOnDeleteEnabled,
    setAutoCleanOnDeleteEnabled,
    open,
    setOpen,
    editingProvider,
    setEditingProvider,
    deleteId,
    setDeleteId,
    clearAssociationId,
    setClearAssociationId,
    modelsOpen,
    setModelsOpen,
    modelsOpenId,
    setModelsOpenId,
    selectedUpstreamModels,
    setSelectedUpstreamModels,
    allModelsOpen,
    setAllModelsOpen,
    allModelsProvider,
    setAllModelsProvider,
    selectedAllModels,
    setSelectedAllModels,
    customModelInput,
    setCustomModelInput,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
    allModelsTestResults,
    setAllModelsTestResults,
    batchTesting,
    setBatchTesting,
    batchTestProgress,
    setBatchTestProgress,
    upstreamTestResults,
    setUpstreamTestResults,
    upstreamBatchTesting,
    setUpstreamBatchTesting,
    upstreamBatchTestProgress,
    setUpstreamBatchTestProgress,
    showApiKey,
    setShowApiKey,
    toggleShowApiKey,
    nameFilter,
    setNameFilter,
    debouncedNameFilter,
    setDebouncedNameFilter,
    typeFilter,
    setTypeFilter,
    availableTypes,
    setAvailableTypes,
    flushNameFilter,
    resetTransient,
  } = useProvidersPageStore((state) => ({
    loading: state.loading,
    setLoading: state.setLoading,
    providers: state.providers,
    setProviders: state.setProviders,
    providerTemplates: state.providerTemplates,
    setProviderTemplates: state.setProviderTemplates,
    clearingAssociation: state.clearingAssociation,
    setClearingAssociation: state.setClearingAssociation,
    modelsLoading: state.modelsLoading,
    setModelsLoading: state.setModelsLoading,
    addingModels: state.addingModels,
    setAddingModels: state.setAddingModels,
    syncingModels: state.syncingModels,
    setSyncingModels: state.setSyncingModels,
    syncingAll: state.syncingAll,
    setSyncingAll: state.setSyncingAll,
    autoAssociateOnAddEnabled: state.autoAssociateOnAddEnabled,
    setAutoAssociateOnAddEnabled: state.setAutoAssociateOnAddEnabled,
    autoCleanOnDeleteEnabled: state.autoCleanOnDeleteEnabled,
    setAutoCleanOnDeleteEnabled: state.setAutoCleanOnDeleteEnabled,
    open: state.providerDialogOpen,
    setOpen: state.setProviderDialogOpen,
    editingProvider: state.editingProvider,
    setEditingProvider: state.setEditingProvider,
    deleteId: state.deleteId,
    setDeleteId: state.setDeleteId,
    clearAssociationId: state.clearAssociationId,
    setClearAssociationId: state.setClearAssociationId,
    modelsOpen: state.modelsOpen,
    setModelsOpen: state.setModelsOpen,
    modelsOpenId: state.modelsOpenId,
    setModelsOpenId: state.setModelsOpenId,
    selectedUpstreamModels: state.selectedUpstreamModels,
    setSelectedUpstreamModels: state.setSelectedUpstreamModels,
    allModelsOpen: state.allModelsOpen,
    setAllModelsOpen: state.setAllModelsOpen,
    allModelsProvider: state.allModelsProvider,
    setAllModelsProvider: state.setAllModelsProvider,
    selectedAllModels: state.selectedAllModels,
    setSelectedAllModels: state.setSelectedAllModels,
    customModelInput: state.customModelInput,
    setCustomModelInput: state.setCustomModelInput,
    allModelsSearchQuery: state.allModelsSearchQuery,
    setAllModelsSearchQuery: state.setAllModelsSearchQuery,
    allModelsTestResults: state.allModelsTestResults,
    setAllModelsTestResults: state.setAllModelsTestResults,
    batchTesting: state.batchTesting,
    setBatchTesting: state.setBatchTesting,
    batchTestProgress: state.batchTestProgress,
    setBatchTestProgress: state.setBatchTestProgress,
    upstreamTestResults: state.upstreamTestResults,
    setUpstreamTestResults: state.setUpstreamTestResults,
    upstreamBatchTesting: state.upstreamBatchTesting,
    setUpstreamBatchTesting: state.setUpstreamBatchTesting,
    upstreamBatchTestProgress: state.upstreamBatchTestProgress,
    setUpstreamBatchTestProgress: state.setUpstreamBatchTestProgress,
    showApiKey: state.showApiKey,
    setShowApiKey: state.setShowApiKey,
    toggleShowApiKey: state.toggleShowApiKey,
    nameFilter: state.nameFilter,
    setNameFilter: state.setNameFilter,
    debouncedNameFilter: state.debouncedNameFilter,
    setDebouncedNameFilter: state.setDebouncedNameFilter,
    typeFilter: state.typeFilter,
    setTypeFilter: state.setTypeFilter,
    availableTypes: state.availableTypes,
    setAvailableTypes: state.setAvailableTypes,
    flushNameFilter: state.flushNameFilter,
    resetTransient: state.resetTransient,
  }));

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

  const { fetchProviders } = useProvidersBootstrap({
    debouncedNameFilter,
    typeFilter,
    setLoading,
    setProviders,
    setProviderTemplates,
    setAvailableTypes,
    setAutoAssociateOnAddEnabled,
    setAutoCleanOnDeleteEnabled,
  });

  const autoActionsFlags = { autoAssociateOnAddEnabled, autoCleanOnDeleteEnabled };

  const {
    allModelsList,
    filteredAllModels,
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
    fetchProviders,
    allModelsProvider,
    setAllModelsProvider,
    setAllModelsOpen,
    selectedAllModels,
    setSelectedAllModels,
    customModelInput,
    setCustomModelInput,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
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

  const { handleSyncAllProviders } = useProviderSyncActions({ setSyncingAll, fetchProviders });

  const { handleSubmitProvider } = useProviderMutations({
    form,
    editingProvider,
    setEditingProvider,
    setOpen,
    fetchProviders,
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
    fetchProviders,
    deleteId,
    setDeleteId,
    clearAssociationId,
    setClearAssociationId,
    clearingAssociation,
    setClearingAssociation,
    autoCleanOnDeleteEnabled,
  });

  const hasFilter = hasActiveProvidersFilter(nameFilter, typeFilter);

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

      <ProviderFormDialog
        open={open}
        onOpenChange={setOpen}
        editingProvider={editingProvider}
        form={form}
        providerTemplates={providerTemplates}
        watchedType={watchedType}
        showApiKey={showApiKey}
        toggleShowApiKey={toggleShowApiKey}
        onSubmit={handleSubmitProvider}
      />

      <AllModelsDialog
        open={allModelsOpen}
        onOpenChange={setAllModelsOpen}
        allModelsProvider={allModelsProvider}
        upstreamStatus={upstreamStatus}
        upstreamModelsList={upstreamModelsList}
        allModelsList={allModelsList}
        filteredAllModels={filteredAllModels}
        allModelsSearchQuery={allModelsSearchQuery}
        setAllModelsSearchQuery={setAllModelsSearchQuery}
        allModelsTestResults={allModelsTestResults}
        batchTesting={batchTesting}
        batchTestProgress={batchTestProgress}
        syncingModels={syncingModels}
        addingModels={addingModels}
        selectedAllModels={selectedAllModels}
        setSelectedAllModels={setSelectedAllModels}
        isAllFilteredSelected={isAllFilteredSelected}
        toggleSelectAllModels={toggleSelectAllModels}
        handleSyncUpstreamModels={handleSyncUpstreamModels}
        handleBatchTestAll={handleBatchTestAll}
        handleBatchTestSelected={handleBatchTestSelected}
        handleCancelBatchTest={handleCancelBatchTest}
        selectAllSuccessful={selectAllSuccessful}
        selectAllFailed={selectAllFailed}
        handleRemoveSelectedModels={handleRemoveSelectedModels}
        handleTestAllModel={handleTestAllModel}
        copyModelName={copyModelName}
        handleRemoveModelFromAll={handleRemoveModelFromAll}
        customModelInput={customModelInput}
        setCustomModelInput={setCustomModelInput}
        handleAddCustomModels={handleAddCustomModels}
      />

      <UpstreamModelsDialog
        open={modelsOpen}
        onOpenChange={setModelsOpen}
        providerName={providers.find((v) => v.ID === modelsOpenId)?.Name}
        modelsOpenId={modelsOpenId}
        modelsLoading={modelsLoading}
        addingModels={addingModels}
        providerModels={providerModels}
        filteredProviderModels={filteredProviderModels}
        cachedModelsCount={getAllModelsForProvider(providers, modelsOpenId || 0).length}
        savedModelSet={savedModelSet}
        selectedUpstreamModels={selectedUpstreamModels}
        setSelectedUpstreamModels={setSelectedUpstreamModels}
        upstreamTestResults={upstreamTestResults}
        upstreamBatchTesting={upstreamBatchTesting}
        upstreamBatchTestProgress={upstreamBatchTestProgress}
        selectableModelIds={selectableModelIds}
        isAllSelectableChecked={isAllSelectableChecked}
        toggleSelectAll={toggleSelectAll}
        handleBatchTestUpstreamAll={handleBatchTestUpstreamAll}
        handleBatchTestUpstreamSelected={handleBatchTestUpstreamSelected}
        handleCancelUpstreamBatchTest={handleCancelUpstreamBatchTest}
        selectUpstreamSuccessful={selectUpstreamSuccessful}
        selectUpstreamFailed={selectUpstreamFailed}
        refreshUpstreamModels={refreshUpstreamModels}
        handleUpstreamSearchChange={handleUpstreamSearchChange}
        handleAddUpstreamToAll={handleAddUpstreamToAll}
        handleTestUpstreamModel={handleTestUpstreamModel}
        copyModelName={copyModelName}
      />
    </div>
  );
}
 
