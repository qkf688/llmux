import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  clearModelSyncLogs,
  clearModelSyncErrorLogs,
  deleteModelSyncLogs,
  getModelSyncLogs,
  getModelSyncStats,
  getRecentAddedModels,
  getProviders,
  syncAllProviderModels,
  updateProvider,
  type ModelSyncLog,
} from "@/lib/api";
import { toast } from "sonner";
import type { ModelSyncTab } from "../types";
import {
  selectModelSyncLogsActiveTab,
  selectModelSyncLogsClearingErrors,
  selectModelSyncLogsClearDialogOpen,
  selectModelSyncLogsDetailLog,
  selectModelSyncLogsErrorsLoading,
  selectModelSyncLogsLoading,
  selectModelSyncLogsLogs,
  selectModelSyncLogsPage,
  selectModelSyncLogsProvidersById,
  selectModelSyncLogsRecentErrors,
  selectModelSyncLogsRecentLoading,
  selectModelSyncLogsRecentModels,
  selectModelSyncLogsSelectedErrorProviders,
  selectModelSyncLogsSelectedLogs,
  selectModelSyncLogsShowUnchanged,
  selectModelSyncLogsStats,
  selectModelSyncLogsStatsLoading,
  selectModelSyncLogsSyncing,
  selectModelSyncLogsSyncTime,
  selectModelSyncLogsTogglingProviderIds,
  selectModelSyncLogsTotalPages,
  selectResetModelSyncLogsTransient,
  selectSetModelSyncLogsActiveTab,
  selectSetModelSyncLogsClearingErrors,
  selectSetModelSyncLogsClearDialogOpen,
  selectSetModelSyncLogsDetailLog,
  selectSetModelSyncLogsErrorsLoading,
  selectSetModelSyncLogsLoading,
  selectSetModelSyncLogsLogs,
  selectSetModelSyncLogsPage,
  selectSetModelSyncLogsProvidersById,
  selectSetModelSyncLogsRecentErrors,
  selectSetModelSyncLogsRecentLoading,
  selectSetModelSyncLogsRecentModels,
  selectSetModelSyncLogsSelectedErrorProviders,
  selectSetModelSyncLogsSelectedLogs,
  selectSetModelSyncLogsShowUnchanged,
  selectSetModelSyncLogsStats,
  selectSetModelSyncLogsStatsLoading,
  selectSetModelSyncLogsSyncing,
  selectSetModelSyncLogsSyncTime,
  selectSetModelSyncLogsTogglingProviderIds,
  selectSetModelSyncLogsTotalPages,
  useModelSyncLogsPageStore,
} from "@/stores/model-sync-logs";

const LOG_PAGE_SIZE = 20;

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

export function useModelSyncLogsPage() {
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const activeTab = useModelSyncLogsPageStore(selectModelSyncLogsActiveTab);
  const logs = useModelSyncLogsPageStore(selectModelSyncLogsLogs);
  const recentModels = useModelSyncLogsPageStore(selectModelSyncLogsRecentModels);
  const recentErrors = useModelSyncLogsPageStore(selectModelSyncLogsRecentErrors);
  const syncTime = useModelSyncLogsPageStore(selectModelSyncLogsSyncTime);
  const stats = useModelSyncLogsPageStore(selectModelSyncLogsStats);
  const providersById = useModelSyncLogsPageStore(selectModelSyncLogsProvidersById);
  const loading = useModelSyncLogsPageStore(selectModelSyncLogsLoading);
  const recentLoading = useModelSyncLogsPageStore(selectModelSyncLogsRecentLoading);
  const errorsLoading = useModelSyncLogsPageStore(selectModelSyncLogsErrorsLoading);
  const syncing = useModelSyncLogsPageStore(selectModelSyncLogsSyncing);
  const statsLoading = useModelSyncLogsPageStore(selectModelSyncLogsStatsLoading);
  const selectedLogs = useModelSyncLogsPageStore(selectModelSyncLogsSelectedLogs);
  const selectedErrorProviders = useModelSyncLogsPageStore(selectModelSyncLogsSelectedErrorProviders);
  const page = useModelSyncLogsPageStore(selectModelSyncLogsPage);
  const totalPages = useModelSyncLogsPageStore(selectModelSyncLogsTotalPages);
  const showUnchanged = useModelSyncLogsPageStore(selectModelSyncLogsShowUnchanged);
  const togglingProviderIds = useModelSyncLogsPageStore(selectModelSyncLogsTogglingProviderIds);
  const detailLog = useModelSyncLogsPageStore(selectModelSyncLogsDetailLog);
  const clearDialogOpen = useModelSyncLogsPageStore(selectModelSyncLogsClearDialogOpen);
  const clearingErrors = useModelSyncLogsPageStore(selectModelSyncLogsClearingErrors);

  const setActiveTab = useModelSyncLogsPageStore(selectSetModelSyncLogsActiveTab);
  const setLogs = useModelSyncLogsPageStore(selectSetModelSyncLogsLogs);
  const setRecentModels = useModelSyncLogsPageStore(selectSetModelSyncLogsRecentModels);
  const setRecentErrors = useModelSyncLogsPageStore(selectSetModelSyncLogsRecentErrors);
  const setSyncTime = useModelSyncLogsPageStore(selectSetModelSyncLogsSyncTime);
  const setStats = useModelSyncLogsPageStore(selectSetModelSyncLogsStats);
  const setProvidersById = useModelSyncLogsPageStore(selectSetModelSyncLogsProvidersById);
  const setLoading = useModelSyncLogsPageStore(selectSetModelSyncLogsLoading);
  const setRecentLoading = useModelSyncLogsPageStore(selectSetModelSyncLogsRecentLoading);
  const setErrorsLoading = useModelSyncLogsPageStore(selectSetModelSyncLogsErrorsLoading);
  const setSyncing = useModelSyncLogsPageStore(selectSetModelSyncLogsSyncing);
  const setStatsLoading = useModelSyncLogsPageStore(selectSetModelSyncLogsStatsLoading);
  const setSelectedLogs = useModelSyncLogsPageStore(selectSetModelSyncLogsSelectedLogs);
  const setSelectedErrorProviders = useModelSyncLogsPageStore(selectSetModelSyncLogsSelectedErrorProviders);
  const setPage = useModelSyncLogsPageStore(selectSetModelSyncLogsPage);
  const setTotalPages = useModelSyncLogsPageStore(selectSetModelSyncLogsTotalPages);
  const setShowUnchanged = useModelSyncLogsPageStore(selectSetModelSyncLogsShowUnchanged);
  const setTogglingProviderIds = useModelSyncLogsPageStore(selectSetModelSyncLogsTogglingProviderIds);
  const setDetailLog = useModelSyncLogsPageStore(selectSetModelSyncLogsDetailLog);
  const setClearDialogOpen = useModelSyncLogsPageStore(selectSetModelSyncLogsClearDialogOpen);
  const setClearingErrors = useModelSyncLogsPageStore(selectSetModelSyncLogsClearingErrors);
  const resetTransient = useModelSyncLogsPageStore(selectResetModelSyncLogsTransient);

  const fetchStats = useCallback(async () => {
    try {
      setStatsLoading(true);
      const data = await getModelSyncStats();
      setStats(data);
    } catch (error) {
      console.error("加载统计信息失败:", error);
    } finally {
      setStatsLoading(false);
    }
  }, [setStats, setStatsLoading]);

  const fetchLogs = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getModelSyncLogs({
        page,
        page_size: LOG_PAGE_SIZE,
        show_unchanged: showUnchanged,
      });
      setLogs(data.data ?? []);
      setTotalPages(data.pagination?.total_pages ?? 1);
    } catch (error) {
      toast.error(`加载日志失败: ${toErrorMessage(error)}`);
    } finally {
      setLoading(false);
    }
  }, [page, setLoading, setLogs, setTotalPages, showUnchanged]);

  const fetchRecentModels = useCallback(async () => {
    try {
      setRecentLoading(true);
      const data = await getRecentAddedModels();
      setRecentModels(data.data ?? []);
      setSyncTime(data.sync_time || "");
    } catch (error) {
      toast.error(`加载最近新增模型失败: ${toErrorMessage(error)}`);
    } finally {
      setRecentLoading(false);
    }
  }, [setRecentLoading, setRecentModels, setSyncTime]);

  const fetchRecentErrors = useCallback(async () => {
    try {
      setErrorsLoading(true);
      const [logsData, providers] = await Promise.all([
        getModelSyncLogs({ page: 1, page_size: 100, status: "error" }),
        getProviders(),
      ]);

      setRecentErrors(logsData.data ?? []);
      setProvidersById(Object.fromEntries((providers ?? []).map((provider) => [provider.ID, provider])));
    } catch (error) {
      toast.error(`加载最近错误失败: ${toErrorMessage(error)}`);
    } finally {
      setErrorsLoading(false);
    }
  }, [setErrorsLoading, setProvidersById, setRecentErrors]);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    if (activeTab === "logs") {
      void fetchLogs();
    }
  }, [activeTab, fetchLogs]);

  useEffect(() => {
    if (activeTab === "recent") {
      void fetchRecentModels();
    }
  }, [activeTab, fetchRecentModels]);

  useEffect(() => {
    if (activeTab === "errors") {
      void fetchRecentErrors();
    }
  }, [activeTab, fetchRecentErrors]);

  useEffect(
    () => () => {
      if (refreshTimerRef.current) {
        clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
      resetTransient();
    },
    [resetTransient]
  );

  useEffect(() => {
    setSelectedLogs(new Set());
  }, [activeTab, page, setSelectedLogs, showUnchanged]);

  useEffect(() => {
    if (activeTab !== "errors") {
      setSelectedErrorProviders(new Set());
    }
  }, [activeTab, setSelectedErrorProviders]);

  const selectedCount = selectedLogs.size;
  const allSelected = logs.length > 0 && logs.every((log) => selectedLogs.has(log.ID));
  const canDeleteSelected = selectedCount > 0;

  const recentErrorProviderIds = useMemo(
    () => Array.from(new Set(recentErrors.map((log) => log.ProviderID))),
    [recentErrors]
  );
  const selectedErrorProvidersCount = selectedErrorProviders.size;
  const allErrorProvidersSelected =
    recentErrorProviderIds.length > 0 && recentErrorProviderIds.every((id) => selectedErrorProviders.has(id));

  useEffect(() => {
    if (selectedErrorProviders.size === 0) {
      return;
    }

    const allowed = new Set(recentErrorProviderIds);
    setSelectedErrorProviders((previous) => {
      if (previous.size === 0) {
        return previous;
      }

      const next = new Set<number>();
      for (const providerId of previous) {
        if (allowed.has(providerId)) {
          next.add(providerId);
        }
      }

      return next.size === previous.size ? previous : next;
    });
  }, [recentErrorProviderIds, selectedErrorProviders.size, setSelectedErrorProviders]);

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

  const handleToggleSelectAllErrorProviders = () => {
    if (allErrorProvidersSelected) {
      setSelectedErrorProviders(new Set());
      return;
    }
    setSelectedErrorProviders(new Set(recentErrorProviderIds));
  };

  const handleToggleSelectErrorProvider = (providerId: number, checked: boolean) => {
    setSelectedErrorProviders((previous) => {
      const next = new Set(previous);
      if (checked) {
        next.add(providerId);
      } else {
        next.delete(providerId);
      }
      return next;
    });
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

  const handleToggleSelectAll = () => {
    if (allSelected) {
      setSelectedLogs(new Set());
      return;
    }

    setSelectedLogs(new Set(logs.map((log) => log.ID)));
  };

  const handleToggleSelectLog = (logId: number, checked: boolean) => {
    setSelectedLogs((previous) => {
      const next = new Set(previous);
      if (checked) {
        next.add(logId);
      } else {
        next.delete(logId);
      }
      return next;
    });
  };

  const handlePageChange = (newPage: number) => {
    if (newPage < 1 || newPage > totalPages || newPage === page) {
      return;
    }
    setPage(newPage);
  };

  const handleShowUnchangedChange = (checked: boolean) => {
    setShowUnchanged(checked);
  };

  const openDetailLog = (log: ModelSyncLog) => {
    setDetailLog(log);
  };

  const closeDetailLog = () => {
    setDetailLog(null);
  };

  const switchToTab = (tab: ModelSyncTab) => {
    setActiveTab(tab);
  };

  const refreshCurrentTab = useCallback(() => {
    if (activeTab === "logs") {
      void fetchLogs();
      return;
    }
    if (activeTab === "recent") {
      void fetchRecentModels();
      return;
    }
    void fetchRecentErrors();
  }, [activeTab, fetchLogs, fetchRecentErrors, fetchRecentModels]);

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

  const isLogSelected = useCallback((logId: number) => selectedLogs.has(logId), [selectedLogs]);
  const isErrorProviderSelected = useCallback(
    (providerId: number) => selectedErrorProviders.has(providerId),
    [selectedErrorProviders]
  );

  const paginationText = useMemo(() => `第 ${page} / ${totalPages} 页`, [page, totalPages]);

  return {
    activeTab,
    logs,
    recentModels,
    recentErrors,
    syncTime,
    stats,
    loading,
    recentLoading,
    errorsLoading,
    syncing,
    statsLoading,
    providersById,
    togglingProviderIds,
    selectedCount,
    allSelected,
    canDeleteSelected,
    selectedErrorProvidersCount,
    allErrorProvidersSelected,
    clearingErrors,
    page,
    totalPages,
    showUnchanged,
    detailLog,
    clearDialogOpen,
    paginationText,
    switchToTab,
    fetchStats,
    refreshCurrentTab,
    handleSyncNow,
    handleToggleProviderModelEndpoint,
    handleDeleteSelected,
    handleClearAll,
    handleClearSelectedErrors,
    handleClearAllErrors,
    handleToggleSelectAll,
    handleToggleSelectAllErrorProviders,
    handleToggleSelectLog,
    handleToggleSelectErrorProvider,
    handlePageChange,
    handleShowUnchangedChange,
    isLogSelected,
    isErrorProviderSelected,
    openDetailLog,
    closeDetailLog,
    setClearDialogOpen,
  };
}
