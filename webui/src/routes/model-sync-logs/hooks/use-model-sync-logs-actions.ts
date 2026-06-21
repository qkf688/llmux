import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toErrorMessage } from "@/lib/errors";
import { toast } from "sonner";
import type { ModelSyncTab } from "../types";
import {
  modelSyncLogsKeys,
  useSyncAllProvidersMutation,
  useDeleteModelSyncLogsMutation,
  useClearModelSyncLogsMutation,
  useClearModelSyncErrorLogsMutation,
  useToggleProviderModelEndpointMutation,
} from "@/hooks/api";
import type { ModelSyncLogsPageState } from "@/stores/model-sync-logs";

type UseModelSyncLogsActionsInput = {
  refreshTimerRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>;
  activeTab: ModelSyncTab;
  page: ModelSyncLogsPageState["page"];
  selectedLogs: ModelSyncLogsPageState["selectedLogs"];
  selectedErrorProviders: ModelSyncLogsPageState["selectedErrorProviders"];
  setSelectedLogs: ModelSyncLogsPageState["setSelectedLogs"];
  setClearDialogOpen: ModelSyncLogsPageState["setClearDialogOpen"];
  setPage: ModelSyncLogsPageState["setPage"];
  setSelectedErrorProviders: ModelSyncLogsPageState["setSelectedErrorProviders"];
  setTogglingProviderIds: ModelSyncLogsPageState["setTogglingProviderIds"];
};

export function useModelSyncLogsActions({
  refreshTimerRef,
  activeTab,
  page,
  selectedLogs,
  selectedErrorProviders,
  setSelectedLogs,
  setClearDialogOpen,
  setPage,
  setSelectedErrorProviders,
  setTogglingProviderIds,
}: UseModelSyncLogsActionsInput) {
  const queryClient = useQueryClient();
  const syncAllMutation = useSyncAllProvidersMutation();
  const deleteLogsMutation = useDeleteModelSyncLogsMutation();
  const clearLogsMutation = useClearModelSyncLogsMutation();
  const clearErrorsMutation = useClearModelSyncErrorLogsMutation();
  const toggleEndpointMutation = useToggleProviderModelEndpointMutation();

  const invalidateCurrentTab = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: modelSyncLogsKeys.stats() });
    if (activeTab === "logs") {
      void queryClient.invalidateQueries({ queryKey: modelSyncLogsKeys.logsList() });
    } else if (activeTab === "recent") {
      void queryClient.invalidateQueries({ queryKey: modelSyncLogsKeys.recent() });
    } else {
      void queryClient.invalidateQueries({ queryKey: modelSyncLogsKeys.errors() });
    }
  }, [activeTab, queryClient]);

  const handleSyncNow = async () => {
    try {
      await syncAllMutation.mutateAsync();
      toast.success("同步已开始，请稍后刷新查看结果");

      if (refreshTimerRef.current) {
        clearTimeout(refreshTimerRef.current);
      }

      refreshTimerRef.current = setTimeout(() => {
        invalidateCurrentTab();
      }, 2000);
    } catch (error) {
      toast.error(`同步失败: ${toErrorMessage(error)}`);
    }
  };

  const handleDeleteSelected = async () => {
    if (selectedLogs.size === 0) {
      return;
    }

    const ids = Array.from(selectedLogs);
    try {
      await deleteLogsMutation.mutateAsync(ids);
      toast.success(`已删除 ${ids.length} 条日志`);
      setSelectedLogs(new Set());
    } catch (error) {
      toast.error(`删除失败: ${toErrorMessage(error)}`);
    }
  };

  const handleClearAll = async () => {
    try {
      await clearLogsMutation.mutateAsync();
      toast.success("已清空所有日志");
      setClearDialogOpen(false);
      setSelectedLogs(new Set());

      if (page !== 1) {
        setPage(1);
      }
    } catch (error) {
      toast.error(`清空失败: ${toErrorMessage(error)}`);
    }
  };

  const handleClearSelectedErrors = async () => {
    if (selectedErrorProviders.size === 0) {
      return;
    }

    const providerIds = Array.from(selectedErrorProviders);
    try {
      const result = await clearErrorsMutation.mutateAsync({ provider_ids: providerIds });
      toast.success(`已清除 ${providerIds.length} 个提供商的错误日志（共 ${result.deleted} 条）`);
      setSelectedErrorProviders(new Set());
    } catch (error) {
      toast.error(`清除失败: ${toErrorMessage(error)}`);
    }
  };

  const handleClearAllErrors = async () => {
    try {
      const result = await clearErrorsMutation.mutateAsync(undefined);
      toast.success(`已清空全部错误日志（共 ${result.deleted} 条）`);
      setSelectedErrorProviders(new Set());
    } catch (error) {
      toast.error(`清空失败: ${toErrorMessage(error)}`);
    }
  };

  const handleToggleProviderModelEndpoint = useCallback(
    async (providerId: number, enabled: boolean) => {
      setTogglingProviderIds((previous) => new Set(previous).add(providerId));
      try {
        await toggleEndpointMutation.mutateAsync({ providerId, enabled });
        toast.success(`模型端点已${enabled ? "开启" : "关闭"}`);
      } catch (error) {
        const message = toErrorMessage(error);
        toast.error(`更新提供商失败: ${message}`);
      } finally {
        setTogglingProviderIds((previous) => {
          const next = new Set(previous);
          next.delete(providerId);
          return next;
        });
      }
    },
    [setTogglingProviderIds, toggleEndpointMutation]
  );

  const syncing = syncAllMutation.isPending;
  const clearingErrors = clearErrorsMutation.isPending;

  return {
    syncing,
    clearingErrors,
    handleSyncNow,
    handleDeleteSelected,
    handleClearAll,
    handleClearSelectedErrors,
    handleClearAllErrors,
    handleToggleProviderModelEndpoint,
  };
}
