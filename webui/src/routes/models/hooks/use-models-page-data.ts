/**
 * Models 页面数据层：API 查询 + store 状态读取 + 派生数据计算。
 */
import { useMemo } from "react";
import { EMPTY_MODELS, EMPTY_PROVIDERS } from "@/lib/empty-constants";
import {
  selectModelsBatchDeleteDialogOpen,
  selectModelsCollapsedProviders,
  selectModelsDeletingModel,
  selectModelsEditingModel,
  selectModelsFormDialogOpen,
  selectModelsModelPickerOpen,
  selectModelsModelSearchQuery,
  selectModelsSearchQuery,
  selectModelsSelectedIds,
  selectModelsSelectedProviderId,
  selectResetModelsTransient,
  selectSetModelsBatchDeleteDialogOpen,
  selectSetModelsCollapsedProviders,
  selectSetModelsDeletingModel,
  selectSetModelsEditingModel,
  selectSetModelsFormDialogOpen,
  selectSetModelsModelPickerOpen,
  selectSetModelsModelSearchQuery,
  selectSetModelsSearchQuery,
  selectSetModelsSelectedIds,
  selectSetModelsSelectedProviderId,
  useModelsPageStore,
} from "@/stores/models";
import { useModels } from "@/hooks/api/use-models";
import { useProviders } from "@/hooks/api/use-providers";
import {
  buildProviderModelGroups,
  filterProviderGroups,
} from "../utils/provider-models";
import { filterModelsByName } from "../utils/selection";

export function useModelsPageData() {
  const { data: models = EMPTY_MODELS, isLoading: loading } = useModels();
  const { data: providersData = EMPTY_PROVIDERS, isLoading: loadingProviderModels } = useProviders();

  // store 状态
  const batchDeleting = useModelsPageStore((s) => s.batchDeleting);
  const setBatchDeleting = useModelsPageStore((s) => s.setBatchDeleting);

  const formDialogOpen = useModelsPageStore(selectModelsFormDialogOpen);
  const setFormDialogOpen = useModelsPageStore(selectSetModelsFormDialogOpen);
  const editingModel = useModelsPageStore(selectModelsEditingModel);
  const setEditingModel = useModelsPageStore(selectSetModelsEditingModel);
  const deletingModel = useModelsPageStore(selectModelsDeletingModel);
  const setDeletingModel = useModelsPageStore(selectSetModelsDeletingModel);
  const selectedIds = useModelsPageStore(selectModelsSelectedIds);
  const setSelectedIds = useModelsPageStore(selectSetModelsSelectedIds);

  const batchDeleteDialogOpen = useModelsPageStore(selectModelsBatchDeleteDialogOpen);
  const setBatchDeleteDialogOpen = useModelsPageStore(selectSetModelsBatchDeleteDialogOpen);
  const selectedProviderId = useModelsPageStore(selectModelsSelectedProviderId);
  const setSelectedProviderId = useModelsPageStore(selectSetModelsSelectedProviderId);
  const collapsedProviders = useModelsPageStore(selectModelsCollapsedProviders);
  const setCollapsedProviders = useModelsPageStore(selectSetModelsCollapsedProviders);
  const modelPickerOpen = useModelsPageStore(selectModelsModelPickerOpen);
  const setModelPickerOpen = useModelsPageStore(selectSetModelsModelPickerOpen);
  const modelSearchQuery = useModelsPageStore(selectModelsModelSearchQuery);
  const setModelSearchQuery = useModelsPageStore(selectSetModelsModelSearchQuery);
  const searchQuery = useModelsPageStore(selectModelsSearchQuery);
  const setSearchQuery = useModelsPageStore(selectSetModelsSearchQuery);

  const resetTransient = useModelsPageStore(selectResetModelsTransient);

  // 派生数据
  const providerModelGroups = useMemo(() => buildProviderModelGroups(providersData), [providersData]);
  const providerModels = useMemo(
    () => providerModelGroups.flatMap((group) => group.models),
    [providerModelGroups],
  );

  const filteredProviderGroups = useMemo(
    () => filterProviderGroups(providerModelGroups, selectedProviderId, modelSearchQuery),
    [providerModelGroups, selectedProviderId, modelSearchQuery],
  );

  const filteredModels = useMemo(
    () => filterModelsByName(models, searchQuery),
    [models, searchQuery],
  );

  return {
    models,
    loading,
    providersData,
    loadingProviderModels,
    providerModelGroups,
    providerModels,
    filteredProviderGroups,
    filteredModels,
    // store state
    batchDeleting,
    setBatchDeleting,
    formDialogOpen,
    setFormDialogOpen,
    editingModel,
    setEditingModel,
    deletingModel,
    setDeletingModel,
    selectedIds,
    setSelectedIds,
    batchDeleteDialogOpen,
    setBatchDeleteDialogOpen,
    selectedProviderId,
    setSelectedProviderId,
    collapsedProviders,
    setCollapsedProviders,
    modelPickerOpen,
    setModelPickerOpen,
    modelSearchQuery,
    setModelSearchQuery,
    searchQuery,
    setSearchQuery,
    resetTransient,
  };
}