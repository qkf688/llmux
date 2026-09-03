import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { syncAllProviderModels } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";

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

      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      // 同步改写分组白名单 → 组织级目录随之变化
      void queryClient.invalidateQueries({ queryKey: providerKeys.catalog() });
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
