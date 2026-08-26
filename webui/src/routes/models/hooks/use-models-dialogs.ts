/**
 * Models 页面对话框控制：表单实例、对话框开关、编辑/创建/删除入口、模型选择器。
 */
import { useEffect } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import type { Model } from "@/lib/api";
import { modelFormSchema, type ModelFormValues } from "../schemas/forms";
import { defaultModelFormValues, toModelFormValues } from "../utils/form-values";
import { buildCollapsedProviderState } from "../utils/provider-models";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

interface UseModelsDialogsParams {
  providerModelGroups: ProviderModelGroup[];
  setCollapsedProviders: (updater: (previous: Record<number, boolean>) => Record<number, boolean>) => void;
  setEditingModel: (model: Model | null) => void;
  setFormDialogOpen: (open: boolean) => void;
  setDeletingModel: (model: Model | null) => void;
  setSelectedProviderId: (id: string) => void;
  setModelPickerOpen: (open: boolean) => void;
  setModelSearchQuery: (query: string) => void;
  providerModels: ProviderModelWithOwner[];
}

export function useModelsDialogs({
  providerModelGroups,
  setCollapsedProviders,
  setEditingModel,
  setFormDialogOpen,
  setDeletingModel,
  setSelectedProviderId,
  setModelPickerOpen,
  setModelSearchQuery,
  providerModels,
}: UseModelsDialogsParams) {
  const form = useForm<ModelFormValues>({
    resolver: zodResolver(modelFormSchema),
    defaultValues: { ...defaultModelFormValues },
  });

  // 折叠状态同步：provider 列表变化后重建折叠状态
  useEffect(() => {
    setCollapsedProviders((previous) => buildCollapsedProviderState(providerModelGroups, previous));
  }, [providerModelGroups, setCollapsedProviders]);

  const handleSelectProviderModel = (modelId: string) => {
    form.setValue("name", modelId);
    setModelPickerOpen(false);
    setModelSearchQuery("");
  };

  const toggleProviderCollapse = (providerId: number) => {
    setCollapsedProviders((previous) => ({
      ...previous,
      [providerId]: !previous[providerId],
    }));
  };

  const openModelPicker = () => {
    if (providerModels.length === 0) {
      toast.error('暂无任何"全部模型"，请先在提供商管理页同步或添加模型');
      return;
    }

    setModelPickerOpen(true);
  };

  const openEditDialog = (model: Model) => {
    setEditingModel(model);
    form.reset(toModelFormValues(model));
    setFormDialogOpen(true);
  };

  const openCreateDialog = () => {
    setEditingModel(null);
    form.reset(defaultModelFormValues);
    setSelectedProviderId("all");
    setFormDialogOpen(true);
  };

  const openDeleteDialog = (model: Model) => {
    setDeletingModel(model);
  };

  return {
    form,
    handleSelectProviderModel,
    toggleProviderCollapse,
    openModelPicker,
    openEditDialog,
    openCreateDialog,
    openDeleteDialog,
  };
}