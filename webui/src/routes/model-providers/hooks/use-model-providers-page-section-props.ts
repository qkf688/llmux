import type { ModelWithProvider } from "@/lib/api";
import type { SectionPropsContext } from "./use-model-providers-page-context";

/**
 * Section props 装配：把上下文对象组装成 4 组 UI 区块 props。
 * 入参为上下文对象（学 models 的 ReturnType 模式），不再接收 65+ 扁平字段。
 */
export function useModelProvidersPageSectionProps(ctx: SectionPropsContext) {
  const { store, filters, localState, associationStatus, batch, operationScope, preview, modelChange, pageActions, mutations, statusToggle, testing, associationDialog, templateEditor, blacklist, selectedModel, isGlobalScope } = ctx;

  const associationFilterPanelProps = {
    filterPanelOpen: store.filterPanelOpen,
    onFilterPanelOpenChange: store.setFilterPanelOpen,
    activeFilterCount: filters.activeFilterCount,
    selectedModelId: localState.selectedModelId,
    models: localState.models,
    onModelChange: modelChange.handleModelChange,
    selectedProviderType: store.selectedProviderType,
    onSelectedProviderTypeChange: store.setSelectedProviderType,
    selectedProviderFilter: store.selectedProviderFilter,
    onSelectedProviderFilterChange: store.setSelectedProviderFilter,
    selectedStatusFilter: store.selectedStatusFilter,
    onSelectedStatusFilterChange: store.setSelectedStatusFilter,
    searchKeyword: store.searchKeyword,
    onSearchKeywordChange: store.setSearchKeyword,
    providers: localState.providers,
    providerTypes: filters.providerTypes,
    selectedAssociationCount: store.selectedAssociationIds.length,
    batchUpdatingStatus: store.batchUpdatingStatus,
    batchActionSheetOpen: store.batchActionSheetOpen,
    onBatchActionSheetOpenChange: store.setBatchActionSheetOpen,
    batchCapabilitiesDialogOpen: store.batchCapabilitiesDialogOpen,
    onBatchCapabilitiesDialogOpenChange: store.setBatchCapabilitiesDialogOpen,
    batchUpdatingCapabilities: store.batchUpdatingCapabilities,
    onBatchUpdateCapabilities: batch.handleBatchUpdateCapabilities,
    batchTesting: store.batchTesting,
    filteredAssociationCount: filters.filteredModelProviders.length,
    associationTestResults: store.associationTestResults,
    batchDeleteDialogOpen: store.batchDeleteDialogOpen,
    onBatchDeleteDialogOpenChange: store.setBatchDeleteDialogOpen,
    batchDeleting: store.batchDeleting,
    onBatchDeleteConfirm: batch.handleBatchDeleteAssociations,
    onBatchUpdateStatus: batch.handleBatchUpdateStatus,
    onBatchTestSelected: batch.handleBatchTestSelected,
    onBatchTestAll: batch.handleBatchTestAll,
    onSelectAllSuccessful: batch.selectAllSuccessful,
    onSelectAllFailed: batch.selectAllFailed,
    onToggleTemplateEditor: templateEditor.handleToggleTemplateEditor,
    onOpenBlacklistDialog: blacklist.openBlacklistDialog,
    onAutoAssociate: preview.handleAutoAssociate,
    onCleanInvalid: preview.handleCleanInvalid,
    onOpenCreateDialog: associationDialog.openCreateDialog,
    // ActionMenuDialog（操作菜单）
    actionMenuProps: {
      operationScope: store.operationScope,
      onOperationScopeChange: store.setOperationScope,
      selectedModelName: isGlobalScope ? "全部" : (selectedModel?.Name ?? "未选择"),
      selectedModelId: localState.selectedModelId,
      resettingWeights: store.resettingWeights,
      resettingPriorities: store.resettingPriorities,
      enablingAssociations: store.enablingAssociations,
      onResetWeights: operationScope.handleResetWeights,
      onResetPriorities: operationScope.handleResetPriorities,
      onEnableAssociations: operationScope.handleEnableAssociations,
      onToggleTemplateEditor: templateEditor.handleToggleTemplateEditor,
      onOpenBlacklistDialog: blacklist.openBlacklistDialog,
      onAutoAssociate: preview.handleAutoAssociate,
      onCleanInvalid: preview.handleCleanInvalid,
    },
  };

  const batchTestProgressCardProps = {
    batchTesting: store.batchTesting,
    batchTestProgress: store.batchTestProgress,
    associationTestResults: store.associationTestResults,
    onCancel: batch.handleCancelBatchTest,
    onClear: batch.clearBatchTestResults,
    onSelectSuccess: batch.selectAllSuccessful,
    onSelectFailed: batch.selectAllFailed,
  };

  const associationListSectionProps = {
    loading: localState.loading,
    selectedModelId: localState.selectedModelId,
    hasAssociationFilter: filters.hasAssociationFilter,
    associations: filters.filteredModelProviders as ModelWithProvider[],
    providers: localState.providers,
    selectedAssociationIds: store.selectedAssociationIds,
    isAllSelected: filters.isAllAssociationsSelected,
    isPartialSelected: filters.isPartialAssociationsSelected,
    providerStatus: associationStatus.providerStatus,
    healthStatus: associationStatus.healthStatus,
    statusUpdating: localState.statusUpdating,
    associationTestResults: store.associationTestResults,
    deleteId: store.deleteId,
    onSelectAll: batch.handleSelectAllAssociations,
    onSelectOne: batch.handleSelectOneAssociation,
    onRefreshStatus: pageActions.refreshStatus,
    onToggleStatus: statusToggle.handleStatusToggle,
    onEdit: associationDialog.openEditDialog,
    onOpenDelete: pageActions.openDeleteDialog,
    onDeleteDialogChange: pageActions.handleDeleteDialogChange,
    onDeleteConfirm: mutations.handleDelete,
    onTest: testing.handleTest,
  };

  return {
    associationFilterPanelProps,
    batchTestProgressCardProps,
    associationListSectionProps,
  };
}
