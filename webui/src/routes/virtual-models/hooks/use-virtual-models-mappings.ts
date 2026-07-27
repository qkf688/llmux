import { useMemo, useState } from "react";
import { toast } from "sonner";
import {
  batchCreateVirtualModelMapping,
  batchDeleteVirtualModelMapping,
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
  mappings: VirtualModelMapping[];
  editingMapping: VirtualModelsPageState["editingMapping"];
  selectedModelIds: VirtualModelsPageState["selectedModelIds"];
  batchPriority: VirtualModelsPageState["batchPriority"];
  batchWeight: VirtualModelsPageState["batchWeight"];
  batchEnabled: VirtualModelsPageState["batchEnabled"];
  mappingSearchQuery: VirtualModelsPageState["mappingSearchQuery"];
  selectedMappingIds: VirtualModelsPageState["selectedMappingIds"];
  mappingBatchDeleteDialogOpen: VirtualModelsPageState["mappingBatchDeleteDialogOpen"];
  filteredModels: Model[];
  mappedModelIds: Set<number>;
  mappingForm: UseFormReturn<MappingFormValues>;
  defaults: VirtualModelsBatchDefaults;
  getRealModelName: (modelId: number) => string;
  refreshMappings: (vm?: VirtualModel) => Promise<void>;
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
  setMappingSearchQuery: VirtualModelsPageState["setMappingSearchQuery"];
  setSelectedMappingIds: VirtualModelsPageState["setSelectedMappingIds"];
  setMappingBatchDeleteDialogOpen: VirtualModelsPageState["setMappingBatchDeleteDialogOpen"];
};

export function useVirtualModelsMappings({
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
  setMappingSearchQuery,
  setSelectedMappingIds,
  setMappingBatchDeleteDialogOpen,
}: UseVirtualModelsMappingsInput) {
  const [batchDeleting, setBatchDeleting] = useState(false);

  const filteredMappings = useMemo(() => {
    const keyword = mappingSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return mappings;
    }

    return mappings.filter((mapping) =>
      getRealModelName(mapping.RealModelID).toLowerCase().includes(keyword)
    );
  }, [getRealModelName, mappingSearchQuery, mappings]);

  const filteredMappingIds = useMemo(() => filteredMappings.map((mapping) => mapping.ID), [filteredMappings]);

  const isAllFilteredSelected =
    filteredMappingIds.length > 0 && filteredMappingIds.every((id) => selectedMappingIds.has(id));
  const isSomeFilteredSelected =
    !isAllFilteredSelected && filteredMappingIds.some((id) => selectedMappingIds.has(id));

  const openMappingsDialog = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    setMappingSearchQuery("");
    setSelectedMappingIds(() => new Set<number>());
    setMappingBatchDeleteDialogOpen(false);
    // 显式传入刚选中的 model，避免闭包仍指向上一个 currentVirtualModel
    await refreshMappings(model);
    setMappingsDialogOpen(true);
  };

  const handleMappingsDialogOpenChange = (open: boolean) => {
    setMappingsDialogOpen(open);
    if (!open) {
      setMappingFormDialogOpen(false);
      setMappingBatchDialogOpen(false);
      setEditingMapping(null);
      setMappingSearchQuery("");
      setSelectedMappingIds(() => new Set<number>());
      setMappingBatchDeleteDialogOpen(false);
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
      await refreshMappings();
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
      setSelectedMappingIds((previous) => {
        if (!previous.has(mappingId)) {
          return previous;
        }
        const next = new Set(previous);
        next.delete(mappingId);
        return next;
      });
      await refreshMappings();
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
      await refreshMappings();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`批量添加失败: ${message}`);
    }
  };

  const toggleMappingSelection = (mappingId: number) => {
    setSelectedMappingIds((previous) => {
      const next = new Set(previous);
      if (next.has(mappingId)) {
        next.delete(mappingId);
      } else {
        next.add(mappingId);
      }
      return next;
    });
  };

  const selectAllFilteredMappings = (checked: boolean) => {
    setSelectedMappingIds((previous) => {
      const next = new Set(previous);
      filteredMappingIds.forEach((id) => {
        if (checked) {
          next.add(id);
        } else {
          next.delete(id);
        }
      });
      return next;
    });
  };

  const openBatchDeleteDialog = () => {
    if (selectedMappingIds.size === 0) {
      return;
    }
    setMappingBatchDeleteDialogOpen(true);
  };

  const confirmBatchDelete = async () => {
    if (!currentVirtualModel || selectedMappingIds.size === 0) {
      return;
    }
    if (batchDeleting) {
      return;
    }

    try {
      setBatchDeleting(true);
      const ids = Array.from(selectedMappingIds);
      const result = await batchDeleteVirtualModelMapping(currentVirtualModel.ID, ids);
      toast.success(`已删除 ${result.deleted} 条映射`);
      setMappingBatchDeleteDialogOpen(false);
      setSelectedMappingIds(() => new Set<number>());
      await refreshMappings();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`批量删除失败: ${message}`);
    } finally {
      setBatchDeleting(false);
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
    filteredMappings,
    mappingBatchDeleteDialogOpen,
    setMappingBatchDeleteDialogOpen,
    mappingSearchQuery,
    setMappingSearchQuery,
    selectedMappingIds,
    isAllFilteredSelected,
    isSomeFilteredSelected,
    toggleMappingSelection,
    selectAllFilteredMappings,
    openBatchDeleteDialog,
    confirmBatchDelete,
    batchDeleting,
  };
}
