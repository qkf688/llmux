import { useEffect, useMemo, useRef } from "react";
import {
  type ModelSyncLog,
} from "@/lib/api";
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
import { useModelSyncLogsActions } from "./use-model-sync-logs-actions";
import { useModelSyncLogsFetchers } from "./use-model-sync-logs-fetchers";
import { useModelSyncLogsSelection } from "./use-model-sync-logs-selection";

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

  const { fetchStats, fetchLogs, fetchRecentModels, fetchRecentErrors, refreshCurrentTab } =
    useModelSyncLogsFetchers({
      activeTab,
      page,
      showUnchanged,
      setStats,
      setStatsLoading,
      setLogs,
      setTotalPages,
      setLoading,
      setRecentModels,
      setSyncTime,
      setRecentLoading,
      setRecentErrors,
      setProvidersById,
      setErrorsLoading,
    });

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

  const {
    selectedCount,
    allSelected,
    canDeleteSelected,
    selectedErrorProvidersCount,
    allErrorProvidersSelected,
    handleToggleSelectAll,
    handleToggleSelectLog,
    handlePageChange,
    handleShowUnchangedChange,
    handleToggleSelectAllErrorProviders,
    handleToggleSelectErrorProvider,
    isLogSelected,
    isErrorProviderSelected,
  } = useModelSyncLogsSelection({
    activeTab,
    logs,
    recentErrors,
    selectedLogs,
    selectedErrorProviders,
    page,
    totalPages,
    showUnchanged,
    setSelectedLogs,
    setSelectedErrorProviders,
    setPage,
    setShowUnchanged,
  });

  const {
    handleSyncNow,
    handleDeleteSelected,
    handleClearAll,
    handleClearSelectedErrors,
    handleClearAllErrors,
    handleToggleProviderModelEndpoint,
  } = useModelSyncLogsActions({
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
  });

  const openDetailLog = (log: ModelSyncLog) => {
    setDetailLog(log);
  };

  const closeDetailLog = () => {
    setDetailLog(null);
  };

  const switchToTab = (tab: ModelSyncTab) => {
    setActiveTab(tab);
  };

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
