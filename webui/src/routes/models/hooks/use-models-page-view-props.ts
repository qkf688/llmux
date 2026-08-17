/**
 * Models 页面 props 装配：把 data/selection/dialogs/mutations 的产出组装成 6 组 props。
 */
import { useNavigate } from "react-router-dom";
import type { Model } from "@/lib/api";
import type { useModelsPageData } from "./use-models-page-data";
import type { useModelsSelection } from "./use-models-selection";
import type { useModelsDialogs } from "./use-models-dialogs";
import type { useModelsMutations } from "./use-models-mutations";

interface UseModelsPageViewPropsParams {
  data: ReturnType<typeof useModelsPageData>;
  selection: ReturnType<typeof useModelsSelection>;
  dialogs: ReturnType<typeof useModelsDialogs>;
  mutations: ReturnType<typeof useModelsMutations>;
}

export function useModelsPageViewProps({ data, selection, dialogs, mutations }: UseModelsPageViewPropsParams) {
  const navigate = useNavigate();

  const toolbarProps = {
    totalCount: data.models.length,
    searchQuery: data.searchQuery,
    onSearchQueryChange: data.setSearchQuery,
    selectedCount: data.selectedIds.length,
    batchDeleteDialogOpen: data.batchDeleteDialogOpen,
    onBatchDeleteDialogOpenChange: data.setBatchDeleteDialogOpen,
    batchDeleting: data.batchDeleting,
    onOpenBatchSettings: () => data.setBatchSettingsDialogOpen(true),
    onConfirmBatchDelete: () => {
      void mutations.handleBatchDelete();
    },
    onOpenCreateDialog: dialogs.openCreateDialog,
  };

  const listSectionProps = {
    loading: data.loading,
    totalCount: data.models.length,
    models: data.filteredModels,
    selectedIds: data.selectedIds,
    isAllSelected: selection.isAllSelected,
    isPartialSelected: selection.isPartialSelected,
    togglingIOLog: mutations.togglingIOLog,
    togglingAutoAssociate: mutations.togglingAutoAssociate,
    onSelectAll: selection.handleSelectAll,
    onSelectOne: selection.handleSelectOne,
    onToggleIOLog: (model: Model) => {
      void mutations.handleToggleIOLog(model);
    },
    onToggleAutoAssociate: (model: Model, checked: boolean) => {
      void mutations.handleToggleAutoAssociate(model, checked);
    },
    onAssociate: (model: Model) => navigate(`/model-providers?modelId=${model.ID}`),
    onEdit: dialogs.openEditDialog,
    onDelete: dialogs.openDeleteDialog,
  };

  const modelFormDialogProps = {
    open: data.formDialogOpen,
    onOpenChange: data.setFormDialogOpen,
    editingModel: data.editingModel,
    form: dialogs.form,
    providers: data.providersData,
    selectedProviderId: data.selectedProviderId,
    onSelectedProviderIdChange: data.setSelectedProviderId,
    loadingProviderModels: data.loadingProviderModels,
    hasProviderModels: data.providerModels.length > 0,
    onOpenModelPicker: dialogs.openModelPicker,
    onCreate: mutations.handleCreate,
    onUpdate: mutations.handleUpdate,
  };

  const modelPickerDialogProps = {
    open: data.modelPickerOpen,
    onOpenChange: data.setModelPickerOpen,
    selectedProviderId: data.selectedProviderId,
    providers: data.providersData,
    loadingProviderModels: data.loadingProviderModels,
    providerModels: data.providerModels,
    filteredProviderGroups: data.filteredProviderGroups,
    collapsedProviders: data.collapsedProviders,
    searchQuery: data.modelSearchQuery,
    onSearchQueryChange: data.setModelSearchQuery,
    onToggleProviderCollapse: dialogs.toggleProviderCollapse,
    onSelectModel: dialogs.handleSelectProviderModel,
  };

  const batchSettingsDialogProps = {
    open: data.batchSettingsDialogOpen,
    onOpenChange: data.setBatchSettingsDialogOpen,
    selectedCount: data.selectedIds.length,
    maxRetryRange: selection.maxRetryRange,
    timeOutRange: selection.timeOutRange,
    form: dialogs.batchUpdateForm,
    updating: data.batchUpdating,
    onSubmit: mutations.handleBatchUpdate,
  };

  const modelDeleteDialogProps = {
    open: data.deletingModel !== null,
    modelLabel: data.deletingModel?.Name ?? data.deletingModel?.ID ?? "",
    onOpenChange: (open: boolean) => {
      if (!open) {
        data.setDeletingModel(null);
      }
    },
    onConfirm: () => {
      void mutations.handleDelete();
    },
  };

  return {
    toolbarProps,
    listSectionProps,
    modelFormDialogProps,
    modelPickerDialogProps,
    batchSettingsDialogProps,
    modelDeleteDialogProps,
  };
}
