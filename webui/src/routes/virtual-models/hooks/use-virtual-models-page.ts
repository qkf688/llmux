import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import {
  batchCreateVirtualModelMapping,
  createVirtualModel,
  createVirtualModelMapping,
  deleteVirtualModel,
  deleteVirtualModelMapping,
  getModels,
  getProviders,
  getVirtualModelMappings,
  getVirtualModels,
  updateProvider,
  updateVirtualModel,
  updateVirtualModelMapping,
} from "@/lib/api";
import type { Model, Provider, VirtualModel, VirtualModelMapping } from "@/lib/api";
import {
  defaultMappingFormValues,
  defaultVirtualModelFormValues,
  mappingFormSchema,
  virtualModelFormSchema,
  type MappingFormValues,
  type VirtualModelFormValues,
} from "../schemas/forms";
import type { VirtualModelStrategy } from "../types";

const extractErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

const toVirtualModelStrategy = (strategy: string): VirtualModelStrategy => {
  if (strategy === "round_robin" || strategy === "random") {
    return strategy;
  }
  return "priority";
};

export function useVirtualModelsPage() {
  const [loading, setLoading] = useState(true);
  const [virtualModels, setVirtualModels] = useState<VirtualModel[]>([]);
  const [realModels, setRealModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [blacklistedProviders, setBlacklistedProviders] = useState<Provider[]>([]);

  const [modelDialogOpen, setModelDialogOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<VirtualModel | null>(null);
  const [modelToDeleteId, setModelToDeleteId] = useState<number | null>(null);

  const [mappingsDialogOpen, setMappingsDialogOpen] = useState(false);
  const [currentVirtualModel, setCurrentVirtualModel] = useState<VirtualModel | null>(null);
  const [mappings, setMappings] = useState<VirtualModelMapping[]>([]);
  const [mappingFormDialogOpen, setMappingFormDialogOpen] = useState(false);
  const [editingMapping, setEditingMapping] = useState<VirtualModelMapping | null>(null);

  const [mappingBatchDialogOpen, setMappingBatchDialogOpen] = useState(false);
  const [selectedModelIds, setSelectedModelIds] = useState<number[]>([]);
  const [batchPriority, setBatchPriority] = useState(defaultMappingFormValues.priority);
  const [batchWeight, setBatchWeight] = useState(defaultMappingFormValues.weight);
  const [batchEnabled, setBatchEnabled] = useState(defaultMappingFormValues.enabled);
  const [modelSearchQuery, setModelSearchQuery] = useState("");

  const [blacklistDialogOpen, setBlacklistDialogOpen] = useState(false);
  const [providerSelectorDialogOpen, setProviderSelectorDialogOpen] = useState(false);
  const [selectedProviderIds, setSelectedProviderIds] = useState<number[]>([]);
  const [providerSearchQuery, setProviderSearchQuery] = useState("");

  const virtualModelForm = useForm<VirtualModelFormValues>({
    resolver: zodResolver(virtualModelFormSchema),
    defaultValues: { ...defaultVirtualModelFormValues },
  });

  const mappingForm = useForm<MappingFormValues>({
    resolver: zodResolver(mappingFormSchema),
    defaultValues: { ...defaultMappingFormValues },
  });

  useEffect(() => {
    void fetchInitialData();
  }, []);

  const refreshProvidersState = async () => {
    const latestProviders = await getProviders();
    setProviders(latestProviders);
    setBlacklistedProviders(latestProviders.filter((provider) => provider.blacklisted));
  };

  const fetchInitialData = async () => {
    try {
      setLoading(true);
      const [virtualModelData, realModelData, providerData] = await Promise.all([
        getVirtualModels(),
        getModels(),
        getProviders(),
      ]);
      setVirtualModels(virtualModelData);
      setRealModels(realModelData);
      setProviders(providerData);
      setBlacklistedProviders(providerData.filter((provider) => provider.blacklisted));
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`获取数据失败: ${message}`);
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const refreshMappings = async (virtualModelId: number) => {
    const latestMappings = await getVirtualModelMappings(virtualModelId);
    setMappings(latestMappings);
  };

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

  const openCreateVirtualModel = () => {
    setEditingModel(null);
    virtualModelForm.reset({ ...defaultVirtualModelFormValues });
    setModelDialogOpen(true);
  };

  const openEditVirtualModel = (model: VirtualModel) => {
    setEditingModel(model);
    virtualModelForm.reset({
      name: model.Name,
      description: model.Description,
      strategy: toVirtualModelStrategy(model.Strategy),
      max_retry: model.MaxRetry,
      time_out: model.TimeOut,
      io_log: model.IOLog,
      enabled: model.Enabled,
    });
    setModelDialogOpen(true);
  };

  const submitVirtualModel = async (values: VirtualModelFormValues) => {
    try {
      if (editingModel) {
        await updateVirtualModel(editingModel.ID, values);
        toast.success("虚拟模型更新成功");
      } else {
        await createVirtualModel(values);
        toast.success("虚拟模型创建成功");
      }
      setModelDialogOpen(false);
      await fetchInitialData();
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

  const requestDeleteVirtualModel = (modelId: number) => {
    setModelToDeleteId(modelId);
  };

  const closeDeleteDialog = () => {
    setModelToDeleteId(null);
  };

  const confirmDeleteVirtualModel = async () => {
    if (!modelToDeleteId) {
      return;
    }

    try {
      await deleteVirtualModel(modelToDeleteId);
      toast.success("虚拟模型删除成功");
      setModelToDeleteId(null);
      await fetchInitialData();
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    }
  };

  const openMappingsDialog = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      await refreshMappings(model.ID);
      setMappingsDialogOpen(true);
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`获取映射失败: ${message}`);
    }
  };

  const handleMappingsDialogOpenChange = (open: boolean) => {
    setMappingsDialogOpen(open);
    if (!open) {
      setMappingFormDialogOpen(false);
      setMappingBatchDialogOpen(false);
      setEditingMapping(null);
    }
  };

  const openAddMappingDialog = () => {
    setEditingMapping(null);
    mappingForm.reset({ ...defaultMappingFormValues });
    setMappingFormDialogOpen(true);
  };

  const openEditMappingDialog = (mapping: VirtualModelMapping) => {
    setEditingMapping(mapping);
    mappingForm.reset({
      real_model_id: mapping.RealModelID,
      priority: mapping.Priority,
      weight: mapping.Weight,
      enabled: mapping.Enabled,
    });
    setMappingFormDialogOpen(true);
  };

  const submitMapping = async (values: MappingFormValues) => {
    if (!currentVirtualModel) {
      return;
    }

    try {
      if (editingMapping) {
        await updateVirtualModelMapping(currentVirtualModel.ID, editingMapping.ID, values);
        toast.success("映射更新成功");
      } else {
        await createVirtualModelMapping(currentVirtualModel.ID, values);
        toast.success("映射创建成功");
      }
      setMappingFormDialogOpen(false);
      await refreshMappings(currentVirtualModel.ID);
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

  const deleteMapping = async (mappingId: number) => {
    if (!currentVirtualModel) {
      return;
    }

    try {
      await deleteVirtualModelMapping(currentVirtualModel.ID, mappingId);
      toast.success("映射删除成功");
      await refreshMappings(currentVirtualModel.ID);
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    }
  };

  const openBatchMappingDialog = () => {
    setSelectedModelIds([]);
    setBatchPriority(defaultMappingFormValues.priority);
    setBatchWeight(defaultMappingFormValues.weight);
    setBatchEnabled(defaultMappingFormValues.enabled);
    setModelSearchQuery("");
    setMappingBatchDialogOpen(true);
  };

  const toggleBatchModelSelection = (modelId: number) => {
    setSelectedModelIds((previous) =>
      previous.includes(modelId)
        ? previous.filter((id) => id !== modelId)
        : [...previous, modelId]
    );
  };

  const selectAllBatchModels = () => {
    const selectableModels = filteredModels.filter((model) => !mappedModelIds.has(model.ID));
    setSelectedModelIds(selectableModels.map((model) => model.ID));
  };

  const invertBatchModelSelection = () => {
    const selectableModels = filteredModels.filter((model) => !mappedModelIds.has(model.ID));
    setSelectedModelIds((previous) => {
      const previousSet = new Set(previous);
      return selectableModels
        .filter((model) => !previousSet.has(model.ID))
        .map((model) => model.ID);
    });
  };

  const clearBatchModelSelection = () => {
    setSelectedModelIds([]);
  };

  const submitBatchMapping = async () => {
    if (!currentVirtualModel) {
      return;
    }

    if (selectedModelIds.length === 0) {
      toast.error("请至少选择一个真实模型");
      return;
    }

    try {
      const payload = selectedModelIds.map((modelId) => ({
        real_model_id: modelId,
        priority: batchPriority,
        weight: batchWeight,
        enabled: batchEnabled,
      }));
      const result = await batchCreateVirtualModelMapping(currentVirtualModel.ID, payload);

      if (result.success_count > 0) {
        toast.success(`成功添加 ${result.success_count} 个映射`);
      }
      if (result.failed_count > 0) {
        toast.error(`${result.failed_count} 个映射添加失败`);
        result.failed_items.forEach((item) => {
          toast.error(`${getRealModelName(item.real_model_id)}: ${item.reason}`);
        });
      }

      setMappingBatchDialogOpen(false);
      await refreshMappings(currentVirtualModel.ID);
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`批量添加失败: ${message}`);
    }
  };

  const openBlacklistDialog = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      await refreshProvidersState();
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`获取提供商数据失败: ${message}`);
      setBlacklistedProviders(providers.filter((provider) => provider.blacklisted));
    }
    setBlacklistDialogOpen(true);
  };

  const handleBlacklistDialogOpenChange = (open: boolean) => {
    setBlacklistDialogOpen(open);
    if (!open) {
      setProviderSelectorDialogOpen(false);
    }
  };

  const openProviderSelectorDialog = () => {
    setSelectedProviderIds([]);
    setProviderSearchQuery("");
    setProviderSelectorDialogOpen(true);
  };

  const toggleProviderSelection = (providerId: number) => {
    setSelectedProviderIds((previous) =>
      previous.includes(providerId)
        ? previous.filter((id) => id !== providerId)
        : [...previous, providerId]
    );
  };

  const confirmAddBlacklistedProviders = async () => {
    if (selectedProviderIds.length === 0) {
      toast.error("请至少选择一个提供商");
      return;
    }

    try {
      for (const providerId of selectedProviderIds) {
        await updateProvider(providerId, { blacklisted: true });
      }
      toast.success(`成功拉黑 ${selectedProviderIds.length} 个提供商`);
      setProviderSelectorDialogOpen(false);
      await refreshProvidersState();
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

  const removeBlacklistedProvider = async (providerId: number) => {
    try {
      await updateProvider(providerId, { blacklisted: false });
      toast.success("提供商已解除拉黑");
      await refreshProvidersState();
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

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
