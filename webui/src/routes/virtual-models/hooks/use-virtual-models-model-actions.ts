import { toast } from "sonner";
import {
  createVirtualModel,
  deleteVirtualModel,
  updateVirtualModel,
  type VirtualModel,
} from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import type { UseFormReturn } from "react-hook-form";
import type { VirtualModelsPageState } from "@/stores/virtual-models";
import { defaultVirtualModelFormValues, type VirtualModelFormValues } from "../schemas/forms";
import type { VirtualModelStrategy } from "../types";

const toVirtualModelStrategy = (strategy: string): VirtualModelStrategy => {
  if (strategy === "round_robin" || strategy === "random") {
    return strategy;
  }
  return "priority";
};

type UseVirtualModelsModelActionsInput = {
  editingModel: VirtualModelsPageState["editingModel"];
  modelToDeleteId: VirtualModelsPageState["modelToDeleteId"];
  virtualModelForm: UseFormReturn<VirtualModelFormValues>;
  fetchInitialData: () => Promise<void>;
  setEditingModel: VirtualModelsPageState["setEditingModel"];
  setModelDialogOpen: VirtualModelsPageState["setModelDialogOpen"];
  setModelToDeleteId: VirtualModelsPageState["setModelToDeleteId"];
};

export function useVirtualModelsModelActions({
  editingModel,
  modelToDeleteId,
  virtualModelForm,
  fetchInitialData,
  setEditingModel,
  setModelDialogOpen,
  setModelToDeleteId,
}: UseVirtualModelsModelActionsInput) {
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
      const message = toErrorMessage(error);
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
      const message = toErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    }
  };

  return {
    openCreateVirtualModel,
    openEditVirtualModel,
    submitVirtualModel,
    requestDeleteVirtualModel,
    closeDeleteDialog,
    confirmDeleteVirtualModel,
  };
}
