import { zodResolver } from "@hookform/resolvers/zod";
import { useCallback, useMemo } from "react";
import { useForm } from "react-hook-form";
import { useQueryClient } from "@tanstack/react-query";
import {
  defaultMappingFormValues,
  defaultVirtualModelFormValues,
  mappingFormSchema,
  virtualModelFormSchema,
  type MappingFormValues,
  type VirtualModelFormValues,
} from "../schemas/forms";
import {
  DEFAULT_VIRTUAL_MODELS_BATCH,
  selectResetVirtualModelsTransient,
  selectSetVirtualModelsBatchEnabled,
  selectSetVirtualModelsBatchPriority,
  selectSetVirtualModelsBatchWeight,
  selectSetVirtualModelsBlacklistDialogOpen,
  selectSetVirtualModelsCurrentVirtualModel,
  selectSetVirtualModelsEditingMapping,
  selectSetVirtualModelsEditingModel,
  selectSetVirtualModelsMappingBatchDialogOpen,
  selectSetVirtualModelsMappingBatchDeleteDialogOpen,
  selectSetVirtualModelsMappingFormDialogOpen,
  selectSetVirtualModelsMappingSearchQuery,
  selectSetVirtualModelsMappingsDialogOpen,
  selectSetVirtualModelsModelDialogOpen,
  selectSetVirtualModelsModelSearchQuery,
  selectSetVirtualModelsModelToDeleteId,
  selectSetVirtualModelsProviderSearchQuery,
  selectSetVirtualModelsProviderSelectorDialogOpen,
  selectSetVirtualModelsSelectedMappingIds,
  selectSetVirtualModelsSelectedModelIds,
  selectSetVirtualModelsSelectedProviderIds,
  selectVirtualModelsBatchEnabled,
  selectVirtualModelsBatchPriority,
  selectVirtualModelsBatchWeight,
  selectVirtualModelsBlacklistDialogOpen,
  selectVirtualModelsCurrentVirtualModel,
  selectVirtualModelsEditingMapping,
  selectVirtualModelsEditingModel,
  selectVirtualModelsMappingBatchDialogOpen,
  selectVirtualModelsMappingBatchDeleteDialogOpen,
  selectVirtualModelsMappingFormDialogOpen,
  selectVirtualModelsMappingSearchQuery,
  selectVirtualModelsMappingsDialogOpen,
  selectVirtualModelsModelDialogOpen,
  selectVirtualModelsModelSearchQuery,
  selectVirtualModelsModelToDeleteId,
  selectVirtualModelsProviderSearchQuery,
  selectVirtualModelsProviderSelectorDialogOpen,
  selectVirtualModelsSelectedMappingIds,
  selectVirtualModelsSelectedModelIds,
  selectVirtualModelsSelectedProviderIds,
  useVirtualModelsPageStore,
} from "@/stores/virtual-models";
import { useVirtualModels, useVMMappings, virtualModelKeys } from "@/hooks/api/use-virtual-models";
import { useModels } from "@/hooks/api/use-models";
import { useProviders } from "@/hooks/api/use-providers";
import type { VirtualModel } from "@/lib/api";
import { useVirtualModelsBootstrap } from "./use-virtual-models-bootstrap";
import { useVirtualModelsBlacklist } from "./use-virtual-models-blacklist";
import { useVirtualModelsMappings } from "./use-virtual-models-mappings";
import { useVirtualModelsModelActions } from "./use-virtual-models-model-actions";

export function useVirtualModelsPage() {
  const queryClient = useQueryClient();

  const { data: virtualModelsData, isLoading: virtualModelsLoading } = useVirtualModels();
  const { data: realModelsData, isLoading: modelsLoading } = useModels();
  const { data: providersData, isLoading: providersLoading } = useProviders();

  const virtualModels = virtualModelsData ?? [];
  const realModels = useMemo(() => realModelsData ?? [], [realModelsData]);
  const providers = useMemo(() => providersData ?? [], [providersData]);

  const blacklistedProviders = useMemo(
    () => providers.filter((p) => p.blacklisted),
    [providers],
  );

  const modelDialogOpen = useVirtualModelsPageStore(selectVirtualModelsModelDialogOpen);
  const editingModel = useVirtualModelsPageStore(selectVirtualModelsEditingModel);
  const modelToDeleteId = useVirtualModelsPageStore(selectVirtualModelsModelToDeleteId);
  const mappingsDialogOpen = useVirtualModelsPageStore(selectVirtualModelsMappingsDialogOpen);
  const currentVirtualModel = useVirtualModelsPageStore(selectVirtualModelsCurrentVirtualModel);
  const mappingSearchQuery = useVirtualModelsPageStore(selectVirtualModelsMappingSearchQuery);
  const selectedMappingIds = useVirtualModelsPageStore(selectVirtualModelsSelectedMappingIds);
  const mappingBatchDeleteDialogOpen = useVirtualModelsPageStore(selectVirtualModelsMappingBatchDeleteDialogOpen);
  const mappingFormDialogOpen = useVirtualModelsPageStore(selectVirtualModelsMappingFormDialogOpen);
  const editingMapping = useVirtualModelsPageStore(selectVirtualModelsEditingMapping);
  const mappingBatchDialogOpen = useVirtualModelsPageStore(selectVirtualModelsMappingBatchDialogOpen);
  const selectedModelIds = useVirtualModelsPageStore(selectVirtualModelsSelectedModelIds);
  const batchPriority = useVirtualModelsPageStore(selectVirtualModelsBatchPriority);
  const batchWeight = useVirtualModelsPageStore(selectVirtualModelsBatchWeight);
  const batchEnabled = useVirtualModelsPageStore(selectVirtualModelsBatchEnabled);
  const modelSearchQuery = useVirtualModelsPageStore(selectVirtualModelsModelSearchQuery);
  const blacklistDialogOpen = useVirtualModelsPageStore(selectVirtualModelsBlacklistDialogOpen);
  const providerSelectorDialogOpen = useVirtualModelsPageStore(selectVirtualModelsProviderSelectorDialogOpen);
  const selectedProviderIds = useVirtualModelsPageStore(selectVirtualModelsSelectedProviderIds);
  const providerSearchQuery = useVirtualModelsPageStore(selectVirtualModelsProviderSearchQuery);

  const setModelDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsModelDialogOpen);
  const setEditingModel = useVirtualModelsPageStore(selectSetVirtualModelsEditingModel);
  const setModelToDeleteId = useVirtualModelsPageStore(selectSetVirtualModelsModelToDeleteId);
  const setMappingsDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsMappingsDialogOpen);
  const setCurrentVirtualModel = useVirtualModelsPageStore(selectSetVirtualModelsCurrentVirtualModel);
  const setMappingSearchQuery = useVirtualModelsPageStore(selectSetVirtualModelsMappingSearchQuery);
  const setSelectedMappingIds = useVirtualModelsPageStore(selectSetVirtualModelsSelectedMappingIds);
  const setMappingBatchDeleteDialogOpen = useVirtualModelsPageStore(
    selectSetVirtualModelsMappingBatchDeleteDialogOpen
  );
  const setMappingFormDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsMappingFormDialogOpen);
  const setEditingMapping = useVirtualModelsPageStore(selectSetVirtualModelsEditingMapping);
  const setMappingBatchDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsMappingBatchDialogOpen);
  const setSelectedModelIds = useVirtualModelsPageStore(selectSetVirtualModelsSelectedModelIds);
  const setBatchPriority = useVirtualModelsPageStore(selectSetVirtualModelsBatchPriority);
  const setBatchWeight = useVirtualModelsPageStore(selectSetVirtualModelsBatchWeight);
  const setBatchEnabled = useVirtualModelsPageStore(selectSetVirtualModelsBatchEnabled);
  const setModelSearchQuery = useVirtualModelsPageStore(selectSetVirtualModelsModelSearchQuery);
  const setBlacklistDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsBlacklistDialogOpen);
  const setProviderSelectorDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsProviderSelectorDialogOpen);
  const setSelectedProviderIds = useVirtualModelsPageStore(selectSetVirtualModelsSelectedProviderIds);
  const setProviderSearchQuery = useVirtualModelsPageStore(selectSetVirtualModelsProviderSearchQuery);
  const resetTransient = useVirtualModelsPageStore(selectResetVirtualModelsTransient);

  const virtualModelForm = useForm<VirtualModelFormValues>({
    resolver: zodResolver(virtualModelFormSchema),
    defaultValues: { ...defaultVirtualModelFormValues },
  });

  const mappingForm = useForm<MappingFormValues>({
    resolver: zodResolver(mappingFormSchema),
    defaultValues: { ...defaultMappingFormValues },
  });

  const { fetchInitialData, refreshProvidersState } = useVirtualModelsBootstrap({
    resetTransient,
  });

  const { data: mappingsData } = useVMMappings(currentVirtualModel?.ID ?? null);
  const mappings = useMemo(() => mappingsData ?? [], [mappingsData]);

  // 可显式传入 vm，避免 setCurrentVirtualModel 同 tick 内仍读到闭包旧值
  const refreshMappings = useCallback(
    async (vm?: VirtualModel) => {
      const target = vm ?? currentVirtualModel;
      if (target) {
        await queryClient.invalidateQueries({ queryKey: virtualModelKeys.mappings(target.ID) });
      }
    },
    [queryClient, currentVirtualModel],
  );

  const loading = virtualModelsLoading || modelsLoading || providersLoading;

  const filteredModels = useMemo(() => {
    const keyword = modelSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return realModels;
    }

    return realModels.filter((model) => model.Name.toLowerCase().includes(keyword));
  }, [modelSearchQuery, realModels]);

  const filteredProviders = useMemo(() => {
    const keyword = providerSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return providers;
    }

    return providers.filter((provider) => provider.Name.toLowerCase().includes(keyword));
  }, [providerSearchQuery, providers]);

  const mappedModelIds = useMemo(
    () => new Set(mappings.map((mapping) => mapping.RealModelID)),
    [mappings]
  );

  const realModelNameMap = useMemo(
    () => new Map(realModels.map((model) => [model.ID, model.Name])),
    [realModels]
  );

  const getRealModelName = (modelId: number) => realModelNameMap.get(modelId) ?? `ID: ${modelId}`;

  const {
    openCreateVirtualModel,
    openEditVirtualModel,
    submitVirtualModel,
    requestDeleteVirtualModel,
    closeDeleteDialog,
    confirmDeleteVirtualModel,
  } = useVirtualModelsModelActions({
    editingModel,
    modelToDeleteId,
    virtualModelForm,
    fetchInitialData,
    setEditingModel,
    setModelDialogOpen,
    setModelToDeleteId,
  });

  const {
    openMappingsDialog,
    handleMappingsDialogOpenChange,
    openAddMappingDialog,
    openEditMappingDialog,
    submitMapping,
    deleteMapping,
    openBatchMappingDialog,
    toggleBatchModelSelection,
    selectAllBatchModels,
    invertBatchModelSelection,
    clearBatchModelSelection,
    submitBatchMapping,
    filteredMappings,
    mappingBatchDeleteDialogOpen: mappingBatchDeleteDialogOpenFromHook,
    setMappingBatchDeleteDialogOpen: setMappingBatchDeleteDialogOpenFromHook,
    mappingSearchQuery: mappingSearchQueryFromHook,
    setMappingSearchQuery: setMappingSearchQueryFromHook,
    selectedMappingIds: selectedMappingIdsFromHook,
    isAllFilteredSelected,
    isSomeFilteredSelected,
    toggleMappingSelection,
    selectAllFilteredMappings,
    openBatchDeleteDialog,
    confirmBatchDelete,
    batchDeleting,
  } = useVirtualModelsMappings({
    currentVirtualModel,
    mappings,
    editingMapping,
    selectedModelIds,
    batchPriority,
    batchWeight,
    batchEnabled,
    mappingSearchQuery,
    selectedMappingIds,
    mappingBatchDeleteDialogOpen,
    filteredModels,
    mappedModelIds,
    mappingForm,
    defaults: DEFAULT_VIRTUAL_MODELS_BATCH,
    getRealModelName,
    refreshMappings,
    setCurrentVirtualModel,
    setMappingsDialogOpen,
    setMappingFormDialogOpen,
    setMappingBatchDialogOpen,
    setEditingMapping,
    setSelectedModelIds,
    setBatchPriority,
    setBatchWeight,
    setBatchEnabled,
    setModelSearchQuery,
    setMappingSearchQuery,
    setSelectedMappingIds,
    setMappingBatchDeleteDialogOpen,
  });

  const {
    openBlacklistDialog,
    handleBlacklistDialogOpenChange,
    openProviderSelectorDialog,
    toggleProviderSelection,
    confirmAddBlacklistedProviders,
    removeBlacklistedProvider,
  } = useVirtualModelsBlacklist({
    selectedProviderIds,
    refreshProvidersState,
    setCurrentVirtualModel,
    setBlacklistDialogOpen,
    setProviderSelectorDialogOpen,
    setSelectedProviderIds,
    setProviderSearchQuery,
  });

  return {
    loading,
    virtualModels,
    realModels,
    blacklistedProviders,
    filteredModels,
    filteredProviders,
    mappedModelIds,
    currentVirtualModel,
    mappings,
    filteredMappings,
    editingModel,
    editingMapping,
    modelToDeleteId,
    mappingSearchQuery: mappingSearchQueryFromHook,
    selectedMappingIds: selectedMappingIdsFromHook,
    mappingBatchDeleteDialogOpen: mappingBatchDeleteDialogOpenFromHook,
    selectedModelIds,
    selectedProviderIds,
    batchPriority,
    batchWeight,
    batchEnabled,
    modelSearchQuery,
    providerSearchQuery,
    modelDialogOpen,
    mappingsDialogOpen,
    mappingFormDialogOpen,
    mappingBatchDialogOpen,
    blacklistDialogOpen,
    providerSelectorDialogOpen,
    virtualModelForm,
    mappingForm,
    getRealModelName,
    openCreateVirtualModel,
    openEditVirtualModel,
    submitVirtualModel,
    requestDeleteVirtualModel,
    closeDeleteDialog,
    confirmDeleteVirtualModel,
    openMappingsDialog,
    handleMappingsDialogOpenChange,
    openAddMappingDialog,
    openEditMappingDialog,
    submitMapping,
    deleteMapping,
    openBatchMappingDialog,
    toggleBatchModelSelection,
    selectAllBatchModels,
    invertBatchModelSelection,
    clearBatchModelSelection,
    submitBatchMapping,
    isAllFilteredSelected,
    isSomeFilteredSelected,
    toggleMappingSelection,
    selectAllFilteredMappings,
    openBatchDeleteDialog,
    confirmBatchDelete,
    batchDeleting,
    setMappingBatchDeleteDialogOpen: setMappingBatchDeleteDialogOpenFromHook,
    setMappingSearchQuery: setMappingSearchQueryFromHook,
    openBlacklistDialog,
    handleBlacklistDialogOpenChange,
    openProviderSelectorDialog,
    toggleProviderSelection,
    confirmAddBlacklistedProviders,
    removeBlacklistedProvider,
    setModelDialogOpen,
    setMappingFormDialogOpen,
    setMappingBatchDialogOpen,
    setProviderSelectorDialogOpen,
    setBatchPriority,
    setBatchWeight,
    setBatchEnabled,
    setModelSearchQuery,
    setProviderSearchQuery,
  };
}
