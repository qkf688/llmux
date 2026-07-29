/**
 * Models 页面数据层：API 查询 + store 状态读取 + 派生数据计算。
 */
import { useMemo } from "react";
import type { Model, Provider } from "@/lib/api";
import {
  selectModelsBatchDeleteDialogOpen,
  selectModelsBatchSettingsDialogOpen,
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
  selectSetModelsBatchSettingsDialogOpen,
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

/** 稳定空数组，避免 `data = []` 在 loading 时每次 render 产生新引用触发 effect 循环 */
const EMPTY_MODELS: Model[] = [];
const EMPTY_PROVIDERS: Provider[] = [];

export function useModelsPageData() {
  const { data: models = EMPTY_MODELS, isLoading: loading } = useModels();
  const { data: providersData = EMPTY_PROVIDERS, isLoading: loadingProviderModels } = useProviders();

  // store 状态
  const batchDeleting = useModelsPageStore((s) => s.batchDeleting);
  const setBatchDeleting = useModelsPageStore((s) => s.setBatchDeleting);
  const batchUpdating = useModelsPageStore((s) => s.batchUpdating);
  const setBatchUpdating = useModelsPageStore((s) => s.setBatchUpdating);

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
  const batchSettingsDialogOpen = useModelsPageStore(selectModelsBatchSettingsDialogOpen);
  const setBatchSettingsDialogOpen = useModelsPageStore(selectSetModelsBatchSettingsDialogOpen);
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
    batchUpdating,
    setBatchUpdating,
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
    batchSettingsDialogOpen,
    setBatchSettingsDialogOpen,
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