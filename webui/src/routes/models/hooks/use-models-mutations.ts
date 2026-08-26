/**
 * Models 页面变更操作：CRUD + toggle + batch + cache patch。
 */
import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { Model } from "@/lib/api";
import {
  useCreateModel,
  useUpdateModel,
  useDeleteModel,
  useBatchDeleteModels,
  modelKeys,
} from "@/hooks/api/use-models";
import { defaultModelFormValues } from "../utils/form-values";
import { buildModelUpdatePayload } from "../utils/model-update-payload";
import type { ModelFormValues } from "../schemas/forms";
import type { UseFormReturn } from "react-hook-form";

interface UseModelsMutationsParams {
  editingModel: Model | null;
  deletingModel: Model | null;
  selectedIds: number[];
  setFormDialogOpen: (open: boolean) => void;
  setEditingModel: (model: Model | null) => void;
  setDeletingModel: (model: Model | null) => void;
  setSelectedIds: (updater: (previous: number[]) => number[]) => void;
  setBatchDeleteDialogOpen: (open: boolean) => void;
  setBatchDeleting: (value: boolean) => void;
  form: UseFormReturn<ModelFormValues>;
}

export function useModelsMutations({
  editingModel,
  deletingModel,
  selectedIds,
  setFormDialogOpen,
  setEditingModel,
  setDeletingModel,
  setSelectedIds,
  setBatchDeleteDialogOpen,
  setBatchDeleting,
  form,
}: UseModelsMutationsParams) {
  const queryClient = useQueryClient();
  const [togglingIOLog, setTogglingIOLog] = useState<Record<number, boolean>>({});
  const [togglingAutoAssociate, setTogglingAutoAssociate] = useState<Record<number, boolean>>({});

  const batchDeleteMutation = useBatchDeleteModels();
  const createMutation = useCreateModel();
  const updateMutation = useUpdateModel();
  const deleteMutation = useDeleteModel();

  const patchModelInCache = useCallback(
    (modelId: number, patch: Partial<Model>) => {
      queryClient.setQueriesData<Model[]>(
        { queryKey: modelKeys.list() },
        (old) => old?.map((item) => (item.ID === modelId ? { ...item, ...patch } : item)),
      );
    },
    [queryClient],
  );

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

  const handleBatchDelete = async () => {
    if (selectedIds.length === 0) {
      return;
    }

    setBatchDeleting(true);

    try {
      const result = await batchDeleteMutation.mutateAsync(selectedIds);
      toast.success(`成功删除 ${result.deleted} 个模型`);
      setSelectedIds(() => []);
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
          // 不覆盖条件写字段（thinking/auto_associate）：避免 stale cache 值回写覆盖他人修改（后端 *bool 条件写，undefined 被 JSON 省略）
          supports_thinking: undefined,
          auto_associate: undefined,
          // 注：name/remark/io_log 仍随全量 payload 回写（既有 toggle 模式，后端无条件写，故不能只提交单字段）
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
          // 不覆盖 thinking：避免 stale cache 值回写覆盖他人修改（后端 *bool 条件写，undefined 被 JSON 省略）
          supports_thinking: undefined,
          // 注：name/remark/io_log 仍随全量 payload 回写（既有 toggle 模式，后端无条件写，故不能只提交单字段）
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

  return {
    togglingIOLog,
    togglingAutoAssociate,
    handleCreate,
    handleUpdate,
    handleDelete,
    handleBatchDelete,
    handleToggleIOLog,
    handleToggleAutoAssociate,
  };
}