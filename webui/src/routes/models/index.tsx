import { zodResolver } from "@hookform/resolvers/zod";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { Model } from "@/lib/api";
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
import {
  useModels,
  useCreateModel,
  useUpdateModel,
  useDeleteModel,
  useBatchDeleteModels,
  useBatchUpdateModels,
  modelKeys,
} from "@/hooks/api/use-models";
import { useProviders } from "@/hooks/api/use-providers";
import { BatchSettingsDialog } from "./components/dialogs/batch-settings-dialog";
import { ModelDeleteDialog } from "./components/dialogs/model-delete-dialog";
import { ModelFormDialog } from "./components/dialogs/model-form-dialog";
import { ModelPickerDialog } from "./components/dialogs/model-picker-dialog";
import { ModelsListSection } from "./components/sections/models-list-section";
import { ModelsToolbar } from "./components/sections/models-toolbar";
import {
  batchUpdateSchema,
  modelFormSchema,
  type BatchUpdateValues,
  type ModelFormValues,
} from "./schemas/forms";
import {
  defaultBatchUpdateValues,
  defaultModelFormValues,
  toModelFormValues,
} from "./utils/form-values";
import { buildModelUpdatePayload } from "./utils/model-update-payload";
import {
  buildCollapsedProviderState,
  buildProviderModelGroups,
  filterProviderGroups,
} from "./utils/provider-models";
import { calculateSelectedRanges, collectSelectedModels, filterModelsByName } from "./utils/selection";

export default function ModelsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [togglingIOLog, setTogglingIOLog] = useState<Record<number, boolean>>({});
  const [togglingAutoAssociate, setTogglingAutoAssociate] = useState<Record<number, boolean>>({});

  const { data: models = [], isLoading: loading } = useModels();
  const { data: providersData = [], isLoading: loadingProviderModels } = useProviders();

  const providerModelGroups = useMemo(() => buildProviderModelGroups(providersData), [providersData]);
  const providerModels = useMemo(
    () => providerModelGroups.flatMap((group) => group.models),
    [providerModelGroups],
  );

  const batchDeleteMutation = useBatchDeleteModels();
  const batchUpdateMutation = useBatchUpdateModels();
  const createMutation = useCreateModel();
  const updateMutation = useUpdateModel();
  const deleteMutation = useDeleteModel();

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

  const form = useForm<ModelFormValues>({
    resolver: zodResolver(modelFormSchema),
    defaultValues: { ...defaultModelFormValues },
  });

  const batchUpdateForm = useForm<BatchUpdateValues>({
    resolver: zodResolver(batchUpdateSchema),
    defaultValues: { ...defaultBatchUpdateValues },
  });

  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  useEffect(() => {
    setCollapsedProviders((previous) => buildCollapsedProviderState(providerModelGroups, previous));
  }, [providerModelGroups, setCollapsedProviders]);

  useEffect(() => {
    setSelectedIds((previous) => {
      if (previous.length === 0) {
        return previous;
      }

      const validIdSet = new Set(models.map((model) => model.ID));
      const next = previous.filter((id) => validIdSet.has(id));
      return next.length === previous.length ? previous : next;
    });
  }, [models, setSelectedIds]);

  const filteredProviderGroups = useMemo(
    () => filterProviderGroups(providerModelGroups, selectedProviderId, modelSearchQuery),
    [providerModelGroups, selectedProviderId, modelSearchQuery]
  );

  const filteredModels = useMemo(
    () => filterModelsByName(models, searchQuery),
    [models, searchQuery]
  );

  const selectedModels = useMemo(
    () => collectSelectedModels(models, selectedIds),
    [models, selectedIds]
  );

  const { maxRetryRange, timeOutRange } = useMemo(
    () => calculateSelectedRanges(selectedModels),
    [selectedModels]
  );

  const isAllSelected = models.length > 0 && selectedIds.length === models.length;
  const isPartialSelected = selectedIds.length > 0 && selectedIds.length < models.length;

  const patchModelInCache = useCallback(
    (modelId: number, patch: Partial<Model>) => {
      queryClient.setQueriesData<Model[]>(
        { queryKey: modelKeys.list() },
        (old) => old?.map((item) => (item.ID === modelId ? { ...item, ...patch } : item)),
      );
    },
    [queryClient],
  );

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

  const handleCreate = async (values: ModelFormValues) => {
    try {
      await createMutation.mutateAsync(values);
      setFormDialogOpen(false);
      toast.success(`模型: ${values.name} 创建成功`);
      form.reset(defaultModelFormValues);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`创建模型失败: ${message}`);
    }
  };

  const handleUpdate = async (values: ModelFormValues) => {
    if (!editingModel) {
      return;
    }

    try {
      await updateMutation.mutateAsync({ id: editingModel.ID, data: values });
      setFormDialogOpen(false);
      setEditingModel(null);
      toast.success(`模型: ${values.name} 更新成功`);
      form.reset(defaultModelFormValues);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`更新模型失败: ${message}`);
      console.error(error);
    }
  };

  const handleDelete = async () => {
    if (!deletingModel) {
      return;
    }

    try {
      await deleteMutation.mutateAsync(deletingModel.ID);
      setDeletingModel(null);
      toast.success(`模型: ${deletingModel.Name ?? deletingModel.ID} 删除成功`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`删除模型失败: ${message}`);
      console.error(error);
    }
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

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(models.map((model) => model.ID));
      return;
    }

    setSelectedIds([]);
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((previous) => {
      if (checked) {
        return previous.includes(id) ? previous : [...previous, id];
      }

      return previous.filter((selectedId) => selectedId !== id);
    });
  };

  const handleBatchDelete = async () => {
    if (selectedIds.length === 0) {
      return;
    }

    setBatchDeleting(true);

    try {
      const result = await batchDeleteMutation.mutateAsync(selectedIds);
      toast.success(`成功删除 ${result.deleted} 个模型`);
      setSelectedIds([]);
      setBatchDeleteDialogOpen(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`批量删除模型失败: ${message}`);
    } finally {
      setBatchDeleting(false);
    }
  };

  const handleToggleIOLog = async (model: Model) => {
    const modelId = model.ID;
    const newIOLogValue = !model.IOLog;

    setTogglingIOLog((previous) => ({ ...previous, [modelId]: true }));

    try {
      await updateMutation.mutateAsync({
        id: modelId,
        data: {
          ...buildModelUpdatePayload(model),
          io_log: newIOLogValue,
        },
      });

      patchModelInCache(modelId, { IOLog: newIOLogValue });

      toast.success(`模型 ${model.Name} 的 IO 记录已${newIOLogValue ? "开启" : "关闭"}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`切换 IO 记录失败: ${message}`);
    } finally {
      setTogglingIOLog((previous) => {
        const next = { ...previous };
        delete next[modelId];
        return next;
      });
    }
  };

  const handleToggleAutoAssociate = async (model: Model, checked: boolean) => {
    const modelId = model.ID;

    setTogglingAutoAssociate((previous) => ({ ...previous, [modelId]: true }));

    try {
      await updateMutation.mutateAsync({
        id: modelId,
        data: {
          ...buildModelUpdatePayload(model),
          auto_associate: checked,
        },
      });

      patchModelInCache(modelId, { auto_associate: checked });

      toast.success(`模型 "${model.Name}" 的自动关联设置已更新`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`更新自动关联设置失败: ${message}`);
      console.error("更新自动关联设置失败:", error);
    } finally {
      setTogglingAutoAssociate((previous) => {
        const next = { ...previous };
        delete next[modelId];
        return next;
      });
    }
  };

  const handleBatchUpdate = async (values: BatchUpdateValues) => {
    if (selectedIds.length === 0) {
      return;
    }

    setBatchUpdating(true);

    try {
      const params: { ids: number[]; max_retry?: number; time_out?: number } = {
        ids: selectedIds,
      };

      if (values.enableMaxRetry) {
        params.max_retry = values.max_retry;
      }

      if (values.enableTimeOut) {
        params.time_out = values.time_out;
      }

      const result = await batchUpdateMutation.mutateAsync(params);
      toast.success(`成功更新 ${result.updated} 个模型`);
      setSelectedIds([]);
      setBatchSettingsDialogOpen(false);
      batchUpdateForm.reset(defaultBatchUpdateValues);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`批量更新模型失败: ${message}`);
    } finally {
      setBatchUpdating(false);
    }
  };

  const modelFormDialogProps = {
    open: formDialogOpen,
    onOpenChange: setFormDialogOpen,
    editingModel,
    form,
    providers: providersData,
    selectedProviderId,
    onSelectedProviderIdChange: setSelectedProviderId,
    loadingProviderModels,
    hasProviderModels: providerModels.length > 0,
    onOpenModelPicker: openModelPicker,
    onCreate: handleCreate,
    onUpdate: handleUpdate,
  };

  const modelPickerDialogProps = {
    open: modelPickerOpen,
    onOpenChange: setModelPickerOpen,
    selectedProviderId,
    providers: providersData,
    loadingProviderModels,
    providerModels,
    filteredProviderGroups,
    collapsedProviders,
    searchQuery: modelSearchQuery,
    onSearchQueryChange: setModelSearchQuery,
    onToggleProviderCollapse: toggleProviderCollapse,
    onSelectModel: handleSelectProviderModel,
  };

  const batchSettingsDialogProps = {
    open: batchSettingsDialogOpen,
    onOpenChange: setBatchSettingsDialogOpen,
    selectedCount: selectedIds.length,
    maxRetryRange,
    timeOutRange,
    form: batchUpdateForm,
    updating: batchUpdating,
    onSubmit: handleBatchUpdate,
  };

  const modelDeleteDialogProps = {
    open: deletingModel !== null,
    modelLabel: deletingModel?.Name ?? deletingModel?.ID ?? "",
    onOpenChange: (open: boolean) => {
      if (!open) {
        setDeletingModel(null);
      }
    },
    onConfirm: () => {
      void handleDelete();
    },
  };

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <ModelsToolbar
        searchQuery={searchQuery}
        onSearchQueryChange={setSearchQuery}
        selectedCount={selectedIds.length}
        batchDeleteDialogOpen={batchDeleteDialogOpen}
        onBatchDeleteDialogOpenChange={setBatchDeleteDialogOpen}
        batchDeleting={batchDeleting}
        onOpenBatchSettings={() => setBatchSettingsDialogOpen(true)}
        onConfirmBatchDelete={() => {
          void handleBatchDelete();
        }}
        onOpenCreateDialog={openCreateDialog}
      />

      <ModelsListSection
        loading={loading}
        totalCount={models.length}
        models={filteredModels}
        selectedIds={selectedIds}
        isAllSelected={isAllSelected}
        isPartialSelected={isPartialSelected}
        togglingIOLog={togglingIOLog}
        togglingAutoAssociate={togglingAutoAssociate}
        onSelectAll={handleSelectAll}
        onSelectOne={handleSelectOne}
        onToggleIOLog={(model) => {
          void handleToggleIOLog(model);
        }}
        onToggleAutoAssociate={(model, checked) => {
          void handleToggleAutoAssociate(model, checked);
        }}
        onAssociate={(model) => navigate(`/model-providers?modelId=${model.ID}`)}
        onEdit={openEditDialog}
        onDelete={openDeleteDialog}
      />

      <ModelFormDialog {...modelFormDialogProps} />

      <ModelPickerDialog {...modelPickerDialogProps} />

      <BatchSettingsDialog {...batchSettingsDialogProps} />

      <ModelDeleteDialog {...modelDeleteDialogProps} />
    </div>
  );
}
