import { toast } from "sonner";
import {
  batchCreateVirtualModelMapping,
  createVirtualModelMapping,
  deleteVirtualModelMapping,
  updateVirtualModelMapping,
  type VirtualModel,
  type VirtualModelMapping,
} from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import type { UseFormReturn } from "react-hook-form";
import type { VirtualModelsPageState } from "@/stores/virtual-models";
import {
  defaultMappingFormValues,
  type MappingFormValues,
} from "../schemas/forms";
import type { Model } from "@/lib/api";
import type { VirtualModelsBatchDefaults } from "@/stores/virtual-models";

type UseVirtualModelsMappingsInput = {
  currentVirtualModel: VirtualModelsPageState["currentVirtualModel"];
  editingMapping: VirtualModelsPageState["editingMapping"];
  selectedModelIds: VirtualModelsPageState["selectedModelIds"];
  batchPriority: VirtualModelsPageState["batchPriority"];
  batchWeight: VirtualModelsPageState["batchWeight"];
  batchEnabled: VirtualModelsPageState["batchEnabled"];
  filteredModels: Model[];
  mappedModelIds: Set<number>;
  mappingForm: UseFormReturn<MappingFormValues>;
  defaults: VirtualModelsBatchDefaults;
  getRealModelName: (modelId: number) => string;
  refreshMappings: (virtualModelId: number) => Promise<void>;
  setCurrentVirtualModel: VirtualModelsPageState["setCurrentVirtualModel"];
  setMappingsDialogOpen: VirtualModelsPageState["setMappingsDialogOpen"];
  setMappingFormDialogOpen: VirtualModelsPageState["setMappingFormDialogOpen"];
  setMappingBatchDialogOpen: VirtualModelsPageState["setMappingBatchDialogOpen"];
  setEditingMapping: VirtualModelsPageState["setEditingMapping"];
  setSelectedModelIds: VirtualModelsPageState["setSelectedModelIds"];
  setBatchPriority: VirtualModelsPageState["setBatchPriority"];
  setBatchWeight: VirtualModelsPageState["setBatchWeight"];
  setBatchEnabled: VirtualModelsPageState["setBatchEnabled"];
  setModelSearchQuery: VirtualModelsPageState["setModelSearchQuery"];
};

export function useVirtualModelsMappings({
  currentVirtualModel,
  editingMapping,
  selectedModelIds,
  batchPriority,
  batchWeight,
  batchEnabled,
  filteredModels,
  mappedModelIds,
  mappingForm,
  defaults,
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
}: UseVirtualModelsMappingsInput) {
  const openMappingsDialog = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      await refreshMappings(model.ID);
      setMappingsDialogOpen(true);
    } catch (error) {
      const message = toErrorMessage(error);
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
      const message = toErrorMessage(error);
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
      const message = toErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    }
  };

  const openBatchMappingDialog = () => {
    setSelectedModelIds([]);
    setBatchPriority(defaults.priority);
    setBatchWeight(defaults.weight);
    setBatchEnabled(defaults.enabled);
    setModelSearchQuery("");
    setMappingBatchDialogOpen(true);
  };

  const toggleBatchModelSelection = (modelId: number) => {
    setSelectedModelIds((previous) =>
      previous.includes(modelId) ? previous.filter((id) => id !== modelId) : [...previous, modelId]
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
      return selectableModels.filter((model) => !previousSet.has(model.ID)).map((model) => model.ID);
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
      const message = toErrorMessage(error);
      toast.error(`批量添加失败: ${message}`);
    }
  };

  return {
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
  };
}
