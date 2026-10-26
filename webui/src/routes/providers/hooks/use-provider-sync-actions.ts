import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { syncAllProviderModels } from "@/lib/api";
import { invalidateAfterProviderWrite } from "@/hooks/api/use-providers";

type UseProviderSyncActionsInput = {
  setSyncingAll: (syncing: boolean) => void;
};

export function useProviderSyncActions({ setSyncingAll }: UseProviderSyncActionsInput) {
  const queryClient = useQueryClient();

  const handleSyncAllProviders = async () => {
    try {
      setSyncingAll(true);
      const result = await syncAllProviderModels();
      const logs = Array.isArray(result.logs) ? result.logs : [];
      const addedTotal = typeof result.added_total === "number" ? result.added_total : 0;
      const removedTotal = typeof result.removed_total === "number" ? result.removed_total : 0;
      const syncedProviders = typeof result.synced_providers === "number" ? result.synced_providers : logs.length;

      if (logs.length > 0) {
        toast.success(`同步完成：新增 ${addedTotal} 个，删除 ${removedTotal} 个模型`, {
          description: syncedProviders > 0 ? `涉及 ${syncedProviders} 个提供商` : undefined,
        });
      } else {
        toast.info(result.message ?? "没有检测到模型变化");
      }

      // 同步改写分组白名单（→ 组织级目录变化），并新增/删除上游模型；后者在
      // 「自动关联/自动清理」开关开启时经 modelsync.ActionHooks 在后台改写
      // model_with_providers，故提供商列表、目录、关联页三个缓存族一起失效。
      // 后台写库与本处 invalidate 存在时序竞态（refetch 可能早一步），最坏情况是
      // 切页看到一次旧数据、再切回即新——比「永远不刷」可接受得多。
      invalidateAfterProviderWrite(queryClient, { catalog: true });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`同步失败: ${message}`);
      console.error(err);
    } finally {
      setSyncingAll(false);
    }
  };

  return { handleSyncAllProviders };
}
