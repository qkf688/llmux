import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { useForm } from "react-hook-form";
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
  selectSetVirtualModelsBlacklistedProviders,
  selectSetVirtualModelsCurrentVirtualModel,
  selectSetVirtualModelsEditingMapping,
  selectSetVirtualModelsEditingModel,
  selectSetVirtualModelsLoading,
  selectSetVirtualModelsMappingBatchDialogOpen,
  selectSetVirtualModelsMappingFormDialogOpen,
  selectSetVirtualModelsMappings,
  selectSetVirtualModelsMappingsDialogOpen,
  selectSetVirtualModelsModelDialogOpen,
  selectSetVirtualModelsModelSearchQuery,
  selectSetVirtualModelsModelToDeleteId,
  selectSetVirtualModelsProviderSearchQuery,
  selectSetVirtualModelsProviderSelectorDialogOpen,
  selectSetVirtualModelsProviders,
  selectSetVirtualModelsRealModels,
  selectSetVirtualModelsSelectedModelIds,
  selectSetVirtualModelsSelectedProviderIds,
  selectSetVirtualModelsVirtualModels,
  selectVirtualModelsBatchEnabled,
  selectVirtualModelsBatchPriority,
  selectVirtualModelsBatchWeight,
  selectVirtualModelsBlacklistDialogOpen,
  selectVirtualModelsBlacklistedProviders,
  selectVirtualModelsCurrentVirtualModel,
  selectVirtualModelsEditingMapping,
  selectVirtualModelsEditingModel,
  selectVirtualModelsLoading,
  selectVirtualModelsMappingBatchDialogOpen,
  selectVirtualModelsMappingFormDialogOpen,
  selectVirtualModelsMappings,
  selectVirtualModelsMappingsDialogOpen,
  selectVirtualModelsModelDialogOpen,
  selectVirtualModelsModelSearchQuery,
  selectVirtualModelsModelToDeleteId,
  selectVirtualModelsProviderSearchQuery,
  selectVirtualModelsProviderSelectorDialogOpen,
  selectVirtualModelsProviders,
  selectVirtualModelsRealModels,
  selectVirtualModelsSelectedModelIds,
  selectVirtualModelsSelectedProviderIds,
  selectVirtualModelsVirtualModels,
  useVirtualModelsPageStore,
} from "@/stores/virtual-models";
import { useVirtualModelsBootstrap } from "./use-virtual-models-bootstrap";
import { useVirtualModelsBlacklist } from "./use-virtual-models-blacklist";
import { useVirtualModelsMappings } from "./use-virtual-models-mappings";
import { useVirtualModelsModelActions } from "./use-virtual-models-model-actions";

export function useVirtualModelsPage() {
  const loading = useVirtualModelsPageStore(selectVirtualModelsLoading);
  const virtualModels = useVirtualModelsPageStore(selectVirtualModelsVirtualModels);
  const realModels = useVirtualModelsPageStore(selectVirtualModelsRealModels);
  const providers = useVirtualModelsPageStore(selectVirtualModelsProviders);
  const blacklistedProviders = useVirtualModelsPageStore(selectVirtualModelsBlacklistedProviders);
  const modelDialogOpen = useVirtualModelsPageStore(selectVirtualModelsModelDialogOpen);
  const editingModel = useVirtualModelsPageStore(selectVirtualModelsEditingModel);
  const modelToDeleteId = useVirtualModelsPageStore(selectVirtualModelsModelToDeleteId);
  const mappingsDialogOpen = useVirtualModelsPageStore(selectVirtualModelsMappingsDialogOpen);
  const currentVirtualModel = useVirtualModelsPageStore(selectVirtualModelsCurrentVirtualModel);
  const mappings = useVirtualModelsPageStore(selectVirtualModelsMappings);
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

  const setLoading = useVirtualModelsPageStore(selectSetVirtualModelsLoading);
  const setVirtualModels = useVirtualModelsPageStore(selectSetVirtualModelsVirtualModels);
  const setRealModels = useVirtualModelsPageStore(selectSetVirtualModelsRealModels);
  const setProviders = useVirtualModelsPageStore(selectSetVirtualModelsProviders);
  const setBlacklistedProviders = useVirtualModelsPageStore(selectSetVirtualModelsBlacklistedProviders);
  const setMappings = useVirtualModelsPageStore(selectSetVirtualModelsMappings);
  const setModelDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsModelDialogOpen);
  const setEditingModel = useVirtualModelsPageStore(selectSetVirtualModelsEditingModel);
  const setModelToDeleteId = useVirtualModelsPageStore(selectSetVirtualModelsModelToDeleteId);
  const setMappingsDialogOpen = useVirtualModelsPageStore(selectSetVirtualModelsMappingsDialogOpen);
  const setCurrentVirtualModel = useVirtualModelsPageStore(selectSetVirtualModelsCurrentVirtualModel);
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

  const { fetchInitialData, refreshProvidersState, refreshMappings } = useVirtualModelsBootstrap({
    setLoading,
    setVirtualModels,
    setRealModels,
    setProviders,
    setBlacklistedProviders,
    setMappings,
    resetTransient,
  });

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
  } = useVirtualModelsMappings({
    currentVirtualModel,
    editingMapping,
    selectedModelIds,
    batchPriority,
    batchWeight,
    batchEnabled,
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
  });

  const {
    openBlacklistDialog,
    handleBlacklistDialogOpenChange,
    openProviderSelectorDialog,
    toggleProviderSelection,
    confirmAddBlacklistedProviders,
    removeBlacklistedProvider,
  } = useVirtualModelsBlacklist({
    providers,
    selectedProviderIds,
    refreshProvidersState,
    setCurrentVirtualModel,
    setBlacklistedProviders,
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
    editingModel,
    editingMapping,
    modelToDeleteId,
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
