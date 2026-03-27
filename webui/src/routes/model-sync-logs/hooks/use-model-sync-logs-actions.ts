import { useCallback } from "react";
import {
  clearModelSyncErrorLogs,
  clearModelSyncLogs,
  deleteModelSyncLogs,
  syncAllProviderModels,
  updateProvider,
} from "@/lib/api";
import { toast } from "sonner";
import type { ModelSyncTab } from "../types";
import type { ModelSyncLogsPageState } from "@/stores/model-sync-logs";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

type UseModelSyncLogsActionsInput = {
  refreshTimerRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>;
  activeTab: ModelSyncTab;
  page: ModelSyncLogsPageState["page"];
  selectedLogs: ModelSyncLogsPageState["selectedLogs"];
  selectedErrorProviders: ModelSyncLogsPageState["selectedErrorProviders"];
  setSyncing: ModelSyncLogsPageState["setSyncing"];
  setSelectedLogs: ModelSyncLogsPageState["setSelectedLogs"];
  setClearDialogOpen: ModelSyncLogsPageState["setClearDialogOpen"];
  setPage: ModelSyncLogsPageState["setPage"];
  setClearingErrors: ModelSyncLogsPageState["setClearingErrors"];
  setSelectedErrorProviders: ModelSyncLogsPageState["setSelectedErrorProviders"];
  setTogglingProviderIds: ModelSyncLogsPageState["setTogglingProviderIds"];
  setProvidersById: ModelSyncLogsPageState["setProvidersById"];
  fetchStats: () => Promise<void>;
  fetchLogs: () => Promise<void>;
  fetchRecentModels: () => Promise<void>;
  fetchRecentErrors: () => Promise<void>;
};

export function useModelSyncLogsActions({
  refreshTimerRef,
  activeTab,
  page,
  selectedLogs,
  selectedErrorProviders,
  setSyncing,
  setSelectedLogs,
  setClearDialogOpen,
  setPage,
  setClearingErrors,
  setSelectedErrorProviders,
  setTogglingProviderIds,
  setProvidersById,
  fetchStats,
  fetchLogs,
  fetchRecentModels,
  fetchRecentErrors,
}: UseModelSyncLogsActionsInput) {
  const handleSyncNow = async () => {
    try {
      setSyncing(true);
      await syncAllProviderModels();
      toast.success("同步已开始，请稍后刷新查看结果");

      if (refreshTimerRef.current) {
        clearTimeout(refreshTimerRef.current);
      }

      refreshTimerRef.current = setTimeout(() => {
        void fetchStats();
        void fetchLogs();
        if (activeTab === "recent") {
          void fetchRecentModels();
        } else if (activeTab === "errors") {
          void fetchRecentErrors();
        }
      }, 2000);
    } catch (error) {
      toast.error(`同步失败: ${toErrorMessage(error)}`);
    } finally {
      setSyncing(false);
    }
  };

  const handleDeleteSelected = async () => {
    if (selectedLogs.size === 0) {
      return;
    }

    const ids = Array.from(selectedLogs);
    try {
      await deleteModelSyncLogs(ids);
      toast.success(`已删除 ${ids.length} 条日志`);
      setSelectedLogs(new Set());
      await fetchLogs();
    } catch (error) {
      toast.error(`删除失败: ${toErrorMessage(error)}`);
    }
  };

  const handleClearAll = async () => {
    try {
      await clearModelSyncLogs();
      toast.success("已清空所有日志");
      setClearDialogOpen(false);
      setSelectedLogs(new Set());

      if (page !== 1) {
        setPage(1);
      } else {
        await fetchLogs();
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
      setClearingErrors(true);
      const result = await clearModelSyncErrorLogs({ provider_ids: providerIds });
      toast.success(`已清除 ${providerIds.length} 个提供商的错误日志（共 ${result.deleted} 条）`);
      setSelectedErrorProviders(new Set());
      await fetchRecentErrors();
      void fetchStats();
    } catch (error) {
      toast.error(`清除失败: ${toErrorMessage(error)}`);
    } finally {
      setClearingErrors(false);
    }
  };

  const handleClearAllErrors = async () => {
    try {
      setClearingErrors(true);
      const result = await clearModelSyncErrorLogs();
      toast.success(`已清空全部错误日志（共 ${result.deleted} 条）`);
      setSelectedErrorProviders(new Set());
      await fetchRecentErrors();
      void fetchStats();
    } catch (error) {
      toast.error(`清空失败: ${toErrorMessage(error)}`);
    } finally {
      setClearingErrors(false);
    }
  };

  const handleToggleProviderModelEndpoint = useCallback(
    async (providerId: number, enabled: boolean) => {
      setTogglingProviderIds((previous) => new Set(previous).add(providerId));
      try {
        const updated = await updateProvider(providerId, { model_endpoint: enabled });
        setProvidersById((previous) => ({ ...previous, [providerId]: updated }));
        toast.success(`${updated.Name} 模型端点已${enabled ? "开启" : "关闭"}`);
        void fetchStats();
      } catch (error) {
        const message = toErrorMessage(error);
        if (message.includes("Provider not found")) {
          setProvidersById((previous) => {
            if (!(providerId in previous)) {
              return previous;
            }
            const next = { ...previous };
            delete next[providerId];
            return next;
          });
          toast.info("提供商已删除，模型端点无需操作");
          return;
        }
        toast.error(`更新提供商失败: ${message}`);
      } finally {
        setTogglingProviderIds((previous) => {
          const next = new Set(previous);
          next.delete(providerId);
          return next;
        });
      }
    },
    [fetchStats, setProvidersById, setTogglingProviderIds]
  );

  return {
    handleSyncNow,
    handleDeleteSelected,
    handleClearAll,
    handleClearSelectedErrors,
    handleClearAllErrors,
    handleToggleProviderModelEndpoint,
  };
}

