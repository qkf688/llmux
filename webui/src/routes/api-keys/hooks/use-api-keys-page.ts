/**
 * API Keys 页面总编排：TanStack Query 拉列表；弹窗开闭本地 state。
 * 失败提示透传 API 层 Error.message——409 冲突文案（响应体信封）能原样到 toast，
 * 依赖的是 api-keys 模块「一切失败 = Error(可展示文案)」的单错误通道契约。
 */
import { useCallback, useState } from "react";
import { toast } from "sonner";
import {
  useApiKeys,
  useCreateApiKey,
  useUpdateApiKey,
  useDeleteApiKey,
} from "@/hooks/api/use-api-keys";
import type { ApiKeyCreateInput, ApiKeyItem } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";

export function useApiKeysPage() {
  const { data: apiKeys = [], isLoading, isError, error } = useApiKeys();
  const createApiKeyMutation = useCreateApiKey();
  const updateApiKeyMutation = useUpdateApiKey();
  const deleteApiKeyMutation = useDeleteApiKey();

  const [formOpen, setFormOpenState] = useState(false);
  // null = 新建；非 null = 编辑该 key（改名 + 查看/复制明文，值不可改）
  const [editingKey, setEditingKey] = useState<ApiKeyItem | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ApiKeyItem | null>(null);

  const setFormOpen = useCallback((open: boolean) => {
    setFormOpenState(open);
    // 与删除弹窗对称：关窗即释放 editingKey（含明文快照），不留陈旧目标
    if (!open) setEditingKey(null);
  }, []);

  const openCreateForm = useCallback(() => {
    setEditingKey(null);
    setFormOpenState(true);
  }, []);

  const openEditForm = useCallback((apiKey: ApiKeyItem) => {
    setEditingKey(apiKey);
    setFormOpenState(true);
  }, []);

  const handleApiKeySaved = useCallback(
    async (values: ApiKeyCreateInput) => {
      try {
        if (editingKey) {
          // headers 三态转发：弹窗恒带（表单即真相）；undefined（非弹窗调用方）
          // 不带键=后端不改。启停走 toggleApiKey，不经此路径。
          await updateApiKeyMutation.mutateAsync({
            id: editingKey.id,
            data: values.headers === undefined ? { name: values.name } : { name: values.name, headers: values.headers },
          });
          toast.success("API Key 已更新");
        } else {
          await createApiKeyMutation.mutateAsync(values);
          // 引导写进成功 toast：IP+HTTP 环境复制常失败，而新建后用户最需要的就是拿到明文
          toast.success("API Key 已创建；若复制失败，可在其编辑弹窗点「显示明文」手动选中复制");
        }
        setFormOpen(false);
      } catch (err) {
        // 不关弹窗：冲突/校验失败时用户改完可原地重交
        toast.error(`${editingKey ? "更新" : "创建"}失败: ${toErrorMessage(err)}`);
      }
    },
    [editingKey, createApiKeyMutation, updateApiKeyMutation, setFormOpen],
  );

  const toggleApiKey = useCallback(
    async (apiKey: ApiKeyItem) => {
      try {
        await updateApiKeyMutation.mutateAsync({
          id: apiKey.id,
          data: { enabled: !apiKey.enabled },
        });
        toast.success(apiKey.enabled ? "已停用" : "已启用");
      } catch (err) {
        toast.error(`切换失败: ${toErrorMessage(err)}`);
      }
    },
    [updateApiKeyMutation],
  );

  const requestDelete = useCallback((apiKey: ApiKeyItem) => {
    setDeleteTarget(apiKey);
    setDeleteOpen(true);
  }, []);

  const handleDeleteOpenChange = useCallback((open: boolean) => {
    setDeleteOpen(open);
    if (!open) setDeleteTarget(null);
  }, []);

  const handleDeleteConfirm = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await deleteApiKeyMutation.mutateAsync(deleteTarget.id);
      toast.success("API Key 已删除");
      setDeleteOpen(false);
      setDeleteTarget(null);
    } catch (err) {
      toast.error(`删除失败: ${toErrorMessage(err)}`);
    }
  }, [deleteTarget, deleteApiKeyMutation]);

  const isBusy =
    createApiKeyMutation.isPending || updateApiKeyMutation.isPending || deleteApiKeyMutation.isPending;

  return {
    apiKeys,
    isLoading,
    isError,
    error,
    formOpen,
    setFormOpen,
    editingKey,
    openCreateForm,
    openEditForm,
    handleApiKeySaved,
    toggleApiKey,
    deleteOpen,
    deleteKeyName: deleteTarget?.name ?? "",
    setDeleteOpen: handleDeleteOpenChange,
    requestDelete,
    handleDeleteConfirm,
    isSaving: isBusy,
  };
}
