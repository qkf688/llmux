import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { clearProviderAssociations, deleteProvider, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";

type UseProviderDangerActionsInput = {
  providers: Provider[];

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
  deleteId,
  setDeleteId,
  clearAssociationId,
  setClearAssociationId,
  clearingAssociation,
  setClearingAssociation,
  autoCleanOnDeleteEnabled,
}: UseProviderDangerActionsInput) {
  const queryClient = useQueryClient();

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const cancelDeleteDialog = () => {
    setDeleteId(null);
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      const targetProvider = providers.find((provider) => provider.ID === deleteId);
      await deleteProvider(deleteId);
      setDeleteId(null);
      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      const message = `提供商 ${targetProvider?.Name ?? deleteId} 删除成功`;
      if (autoCleanOnDeleteEnabled) {
        toast.success(message, { description: "已触发自动清理无效关联（后台异步）" });
      } else {
        toast.success(message);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除提供商失败: ${message}`);
      console.error(err);
    }
  };

  const openClearAssociationsDialog = (id: number) => {
    setClearAssociationId(id);
  };

  const cancelClearAssociationsDialog = () => {
    setClearAssociationId(null);
  };

  const handleClearAssociations = async () => {
    if (!clearAssociationId) return;
    try {
      setClearingAssociation(true);
      const targetProvider = providers.find((provider) => provider.ID === clearAssociationId);
      const result = await clearProviderAssociations(clearAssociationId);
      setClearAssociationId(null);
      toast.success(`提供商 ${targetProvider?.Name ?? clearAssociationId} 的关联已清除`, {
        description: `共清除了 ${result.deleted_count} 个模型关联`,
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
    openDeleteDialog,
    cancelDeleteDialog,
    handleDelete,
    openClearAssociationsDialog,
    cancelClearAssociationsDialog,
    handleClearAssociations,
  };
}
