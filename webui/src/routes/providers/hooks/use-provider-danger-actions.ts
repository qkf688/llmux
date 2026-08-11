import { toast } from "sonner";
import type { Provider } from "@/lib/api";
import { useClearProviderAssociations, useDeleteProvider } from "@/hooks/api/use-providers";

type UseProviderDangerActionsInput = {
  providers: Provider[];

  autoCleanOnDeleteEnabled: boolean;
};

export function useProviderDangerActions({
  providers,
  autoCleanOnDeleteEnabled,
}: UseProviderDangerActionsInput) {
  const deleteMutation = useDeleteProvider();
  // 删除请求的 in-flight 标志：AlertDialogAction 点击后 Radix 立即关闭弹窗，用户重开菜单再次点击可能对
  // 同一 provider 触发第二次 DELETE。仅作 hook 内 guard 防重复，不再透传给 UI（结果反馈由 toast 承担）
  const deleting = deleteMutation.isPending;

  const clearMutation = useClearProviderAssociations();
  // 同 deleting：清除关联的 in-flight 标志，guard 防重复（弹窗 open 已由 row-actions 本地 state 控制，不再依赖全局 dialog id）
  const clearingAssociation = clearMutation.isPending;

  // 每行显式传 provider.ID：弹窗 open 由 row-actions 本地 state 控制，handler 只认传入的 id，与行解耦
  const handleDelete = async (id: number) => {
    if (!id) return;
    if (deleting) {
      // guard 命中（页面级 in-flight）：静默 return 会让用户以为已执行，给一次性提示
      toast.info("已有删除操作进行中，请稍候");
      return;
    }
    try {
      const targetProvider = providers.find((provider) => provider.ID === id);
      await deleteMutation.mutateAsync(id);
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
    }
  };

  // 同 handleDelete：显式传参，与行解耦
  const handleClearAssociations = async (id: number) => {
    if (!id) return;
    if (clearingAssociation) {
      toast.info("已有清除关联操作进行中，请稍候");
      return;
    }
    try {
      const targetProvider = providers.find((provider) => provider.ID === id);
      const result = await clearMutation.mutateAsync(id);
      toast.success(`提供商 ${targetProvider?.Name ?? id} 的关联已清除`, {
        description: `共清除了 ${result.deleted} 个模型关联`,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`清除关联失败: ${message}`);
      console.error(err);
    }
  };

  return {
    handleDelete,
    handleClearAssociations,
  };
}
