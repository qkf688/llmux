import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  batchDeleteModels,
  batchUpdateModels,
  createModel,
  deleteModel,
  getModels,
  getProviders,
  updateModel,
} from "@/lib/api";
import type { Model, Provider } from "@/lib/api";
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
import type { ProviderModelGroup, ProviderModelWithOwner } from "./types";
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
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [formDialogOpen, setFormDialogOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<Model | null>(null);
  const [deletingModel, setDeletingModel] = useState<Model | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchSettingsDialogOpen, setBatchSettingsDialogOpen] = useState(false);
  const [batchUpdating, setBatchUpdating] = useState(false);
  const [togglingIOLog, setTogglingIOLog] = useState<Record<number, boolean>>({});
  const [togglingAutoAssociate, setTogglingAutoAssociate] = useState<Record<number, boolean>>({});

  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProviderId, setSelectedProviderId] = useState("all");
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [loadingProviderModels, setLoadingProviderModels] = useState(false);
  const [collapsedProviders, setCollapsedProviders] = useState<Record<number, boolean>>({});
  const [modelPickerOpen, setModelPickerOpen] = useState(false);
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [searchQuery, setSearchQuery] = useState("");

  const form = useForm<ModelFormValues>({
    resolver: zodResolver(modelFormSchema),
    defaultValues: { ...defaultModelFormValues },
  });

  const batchUpdateForm = useForm<BatchUpdateValues>({
    resolver: zodResolver(batchUpdateSchema),
    defaultValues: { ...defaultBatchUpdateValues },
  });

  useEffect(() => {
    void fetchModels();
    void fetchProviders();
  }, []);

  useEffect(() => {
    setSelectedIds((previous) => {
      if (previous.length === 0) {
        return previous;
      }

      const validIdSet = new Set(models.map((model) => model.ID));
      const next = previous.filter((id) => validIdSet.has(id));
      return next.length === previous.length ? previous : next;
    });
  }, [models]);

  const fetchModels = async () => {
    try {
      setLoading(true);
      const data = await getModels();
      setModels(data);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`获取模型列表失败: ${message}`);
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const fetchProviders = async () => {
    try {
      setLoadingProviderModels(true);
      const data = await getProviders();
      const groups = buildProviderModelGroups(data);

      setProviders(data);
      setProviderModelGroups(groups);
      setProviderModels(groups.flatMap((group) => group.models));
      setCollapsedProviders((previous) => buildCollapsedProviderState(groups, previous));
    } catch (error) {
      console.error("获取供应商列表失败:", error);
    } finally {
      setLoadingProviderModels(false);
    }
  };

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
      toast.error("暂无任何“全部模型”，请先在提供商管理页同步或添加模型");
      return;
    }

    setModelPickerOpen(true);
  };

  const handleCreate = async (values: ModelFormValues) => {
    try {
      await createModel(values);
      setFormDialogOpen(false);
      toast.success(`模型: ${values.name} 创建成功`);
      form.reset(defaultModelFormValues);
      await fetchModels();
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
      await updateModel(editingModel.ID, values);
      setFormDialogOpen(false);
      setEditingModel(null);
      toast.success(`模型: ${values.name} 更新成功`);
      form.reset(defaultModelFormValues);
      await fetchModels();
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
      await deleteModel(deletingModel.ID);
      setDeletingModel(null);
      await fetchModels();
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
      const result = await batchDeleteModels(selectedIds);
      toast.success(`成功删除 ${result.deleted} 个模型`);
      setSelectedIds([]);
      setBatchDeleteDialogOpen(false);
      await fetchModels();
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
      await updateModel(modelId, {
        ...buildModelUpdatePayload(model),
        io_log: newIOLogValue,
      });

      setModels((previousModels) =>
        previousModels.map((item) => (item.ID === modelId ? { ...item, IOLog: newIOLogValue } : item))
      );

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
      await updateModel(modelId, {
        ...buildModelUpdatePayload(model),
        auto_associate: checked,
      });

      setModels((previousModels) =>
        previousModels.map((item) => (item.ID === modelId ? { ...item, auto_associate: checked } : item))
      );

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

      const result = await batchUpdateModels(params);
      toast.success(`成功更新 ${result.updated} 个模型`);
      setSelectedIds([]);
      setBatchSettingsDialogOpen(false);
      batchUpdateForm.reset(defaultBatchUpdateValues);
      await fetchModels();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(`批量更新模型失败: ${message}`);
    } finally {
      setBatchUpdating(false);
    }
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

      <ModelFormDialog
        open={formDialogOpen}
        onOpenChange={setFormDialogOpen}
        editingModel={editingModel}
        form={form}
        providers={providers}
        selectedProviderId={selectedProviderId}
        onSelectedProviderIdChange={setSelectedProviderId}
        loadingProviderModels={loadingProviderModels}
        hasProviderModels={providerModels.length > 0}
        onOpenModelPicker={openModelPicker}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
      />

      <ModelPickerDialog
        open={modelPickerOpen}
        onOpenChange={setModelPickerOpen}
        selectedProviderId={selectedProviderId}
        providers={providers}
        loadingProviderModels={loadingProviderModels}
        providerModels={providerModels}
        filteredProviderGroups={filteredProviderGroups}
        collapsedProviders={collapsedProviders}
        searchQuery={modelSearchQuery}
        onSearchQueryChange={setModelSearchQuery}
        onToggleProviderCollapse={toggleProviderCollapse}
        onSelectModel={handleSelectProviderModel}
      />

      <BatchSettingsDialog
        open={batchSettingsDialogOpen}
        onOpenChange={setBatchSettingsDialogOpen}
        selectedCount={selectedIds.length}
        maxRetryRange={maxRetryRange}
        timeOutRange={timeOutRange}
        form={batchUpdateForm}
        updating={batchUpdating}
        onSubmit={handleBatchUpdate}
      />

      <ModelDeleteDialog
        open={deletingModel !== null}
        modelLabel={deletingModel?.Name ?? deletingModel?.ID ?? ""}
        onOpenChange={(open) => {
          if (!open) {
            setDeletingModel(null);
          }
        }}
        onConfirm={() => {
          void handleDelete();
        }}
      />
    </div>
  );
}
