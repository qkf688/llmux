/**
 * Models 页面编排：聚合 data / selection / dialogs / mutations / view-props。
 */
import { useEffect } from "react";
import { useModelsPageData } from "./use-models-page-data";
import { useModelsSelection } from "./use-models-selection";
import { useModelsDialogs } from "./use-models-dialogs";
import { useModelsMutations } from "./use-models-mutations";
import { useModelsPageViewProps } from "./use-models-page-view-props";

export function useModelsPage() {
  const data = useModelsPageData();
  const { resetTransient } = data;

  const selection = useModelsSelection({
    models: data.models,
    selectedIds: data.selectedIds,
    setSelectedIds: data.setSelectedIds,
  });

  const dialogs = useModelsDialogs({
    providerModelGroups: data.providerModelGroups,
    setCollapsedProviders: data.setCollapsedProviders,
    setEditingModel: data.setEditingModel,
    setFormDialogOpen: data.setFormDialogOpen,
    setDeletingModel: data.setDeletingModel,
    setSelectedProviderId: data.setSelectedProviderId,
    setModelPickerOpen: data.setModelPickerOpen,
    setModelSearchQuery: data.setModelSearchQuery,
    providerModels: data.providerModels,
  });

  const mutations = useModelsMutations({
    editingModel: data.editingModel,
    deletingModel: data.deletingModel,
    selectedIds: data.selectedIds,
    setFormDialogOpen: data.setFormDialogOpen,
    setEditingModel: data.setEditingModel,
    setDeletingModel: data.setDeletingModel,
    setSelectedIds: data.setSelectedIds,
    setBatchDeleteDialogOpen: data.setBatchDeleteDialogOpen,
    setBatchSettingsDialogOpen: data.setBatchSettingsDialogOpen,
    setBatchDeleting: data.setBatchDeleting,
    setBatchUpdating: data.setBatchUpdating,
    form: dialogs.form,
    batchUpdateForm: dialogs.batchUpdateForm,
  });

  // 卸载时重置 transient 状态
  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  return useModelsPageViewProps({ data, selection, dialogs, mutations });
}
