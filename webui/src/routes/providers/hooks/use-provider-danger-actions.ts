import { useState } from "react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { clearProviderAssociations, deleteProvider, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";

type UseProviderDangerActionsInput = {
  providers: Provider[];

  // 保留原有 dialog id state 用于外部弹窗打开态；handler 内部不再读它，避免跨行覆盖导致错删（见 handleDelete/handleClearAssociations 注释）
  deleteId: number | null;
  setDeleteId: (id: number | null) => void;

  clearAssociationId: number | null;
  setClearAssociationId: (id: number | null) => void;

  clearingAssociation: boolean;
  setClearingAssociation: (clearing: boolean) => void;

  autoCleanOnDeleteEnabled: boolean;
};

export function useProviderDangerActions({
  providers,
  setDeleteId,
  setClearAssociationId,
  clearingAssociation,
  setClearingAssociation,
  autoCleanOnDeleteEnabled,
}: UseProviderDangerActionsInput) {
  const queryClient = useQueryClient();
  // 删除请求的 in-flight 标志：AlertDialogAction 点击后 Radix 默认关闭弹窗，
  // 若用户重开菜单再次点击可能对同一 provider 触发第二次 DELETE。用行内 disabled + hook 标志避免重复提交
  const [deleting, setDeleting] = useState(false);

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const cancelDeleteDialog = () => {
    setDeleteId(null);
  };

  // 每行显式传 provider.ID：避免多行 open 弹窗竞争同一个全局 deleteId 导致错删相邻行
  const handleDelete = async (id: number) => {
    if (!id || deleting) return;
    try {
      setDeleting(true);
      const targetProvider = providers.find((provider) => provider.ID === id);
      await deleteProvider(id);
      setDeleteId(null);
      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      const message = `提供商 ${targetProvider?.Name ?? id} 删除成功`;
      if (autoCleanOnDeleteEnabled) {
        toast.success(message, { description: "已触发自动清理无效关联（后台异步）" });
      } else {
        toast.success(message);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除提供商失败: ${message}`);
      console.error(err);
    } finally {
      setDeleting(false);
    }
  };

  const openClearAssociationsDialog = (id: number) => {
    setClearAssociationId(id);
  };

  const cancelClearAssociationsDialog = () => {
    setClearAssociationId(null);
  };

  // 同 handleDelete：显式传参切断跨行覆盖路径
  const handleClearAssociations = async (id: number) => {
    if (!id) return;
    try {
      setClearingAssociation(true);
      const targetProvider = providers.find((provider) => provider.ID === id);
      const result = await clearProviderAssociations(id);
      setClearAssociationId(null);
      toast.success(`提供商 ${targetProvider?.Name ?? id} 的关联已清除`, {
        description: `共清除了 ${result.deleted} 个模型关联`,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`清除关联失败: ${message}`);
      console.error(err);
    } finally {
      setClearingAssociation(false);
    }
  };

  return {
    clearingAssociation,
    deleting,
    openDeleteDialog,
    cancelDeleteDialog,
    handleDelete,
    openClearAssociationsDialog,
    cancelClearAssociationsDialog,
    handleClearAssociations,
  };
}
