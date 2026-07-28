import { useEffect, useMemo, useRef } from "react";
import type { ModelSyncLog, Provider } from "@/lib/api";
import type { ModelSyncTab } from "../types";
import {
  selectModelSyncLogsActiveTab,
  selectModelSyncLogsClearDialogOpen,
  selectModelSyncLogsDetailLog,
  selectModelSyncLogsPage,
  selectModelSyncLogsSelectedErrorProviders,
  selectModelSyncLogsSelectedLogs,
  selectModelSyncLogsShowUnchanged,
  selectModelSyncLogsTogglingProviderIds,
  selectResetModelSyncLogsTransient,
  selectSetModelSyncLogsActiveTab,
  selectSetModelSyncLogsClearDialogOpen,
  selectSetModelSyncLogsDetailLog,
  selectSetModelSyncLogsPage,
  selectSetModelSyncLogsSelectedErrorProviders,
  selectSetModelSyncLogsSelectedLogs,
  selectSetModelSyncLogsShowUnchanged,
  selectSetModelSyncLogsTogglingProviderIds,
  useModelSyncLogsPageStore,
} from "@/stores/model-sync-logs";
import {
  useModelSyncStatsQuery,
  useModelSyncLogsListQuery,
  useRecentModelsQuery,
  useRecentErrorsQuery,
  useProviders,
} from "@/hooks/api";
import { useModelSyncLogsActions } from "./use-model-sync-logs-actions";
import { useModelSyncLogsSelection } from "./use-model-sync-logs-selection";

export function useModelSyncLogsPage() {
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const activeTab = useModelSyncLogsPageStore(selectModelSyncLogsActiveTab);
  const selectedLogs = useModelSyncLogsPageStore(selectModelSyncLogsSelectedLogs);
  const selectedErrorProviders = useModelSyncLogsPageStore(selectModelSyncLogsSelectedErrorProviders);
  const page = useModelSyncLogsPageStore(selectModelSyncLogsPage);
  const showUnchanged = useModelSyncLogsPageStore(selectModelSyncLogsShowUnchanged);
  const togglingProviderIds = useModelSyncLogsPageStore(selectModelSyncLogsTogglingProviderIds);
  const detailLog = useModelSyncLogsPageStore(selectModelSyncLogsDetailLog);
  const clearDialogOpen = useModelSyncLogsPageStore(selectModelSyncLogsClearDialogOpen);

  const setActiveTab = useModelSyncLogsPageStore(selectSetModelSyncLogsActiveTab);
  const setSelectedLogs = useModelSyncLogsPageStore(selectSetModelSyncLogsSelectedLogs);
  const setSelectedErrorProviders = useModelSyncLogsPageStore(selectSetModelSyncLogsSelectedErrorProviders);
  const setPage = useModelSyncLogsPageStore(selectSetModelSyncLogsPage);
  const setShowUnchanged = useModelSyncLogsPageStore(selectSetModelSyncLogsShowUnchanged);
  const setTogglingProviderIds = useModelSyncLogsPageStore(selectSetModelSyncLogsTogglingProviderIds);
  const setDetailLog = useModelSyncLogsPageStore(selectSetModelSyncLogsDetailLog);
  const setClearDialogOpen = useModelSyncLogsPageStore(selectSetModelSyncLogsClearDialogOpen);
  const resetTransient = useModelSyncLogsPageStore(selectResetModelSyncLogsTransient);

  const statsQuery = useModelSyncStatsQuery();
  const logsQuery = useModelSyncLogsListQuery(page, showUnchanged, { enabled: activeTab === "logs" });
  const recentQuery = useRecentModelsQuery({ enabled: activeTab === "recent" });
  const errorsQuery = useRecentErrorsQuery({ enabled: activeTab === "errors" });
  const { data: providers = [] } = useProviders();

  const logs = logsQuery.data?.data ?? [];
  const totalPages = logsQuery.data?.pagination?.total_pages ?? 1;
  const recentModels = recentQuery.data?.data ?? [];
  const syncTime = recentQuery.data?.sync_time ?? "";
  const recentErrors = errorsQuery.data?.data ?? [];
  const providersById = useMemo(
    () => Object.fromEntries(providers.map((p: Provider) => [p.ID, p])),
    [providers]
  );

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
    syncing,
    clearingErrors,
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
    setSelectedLogs,
    setClearDialogOpen,
    setPage,
    setSelectedErrorProviders,
    setTogglingProviderIds,
    providersById,
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
    stats: statsQuery.data ?? null,
    loading: logsQuery.isLoading,
    recentLoading: recentQuery.isLoading,
    errorsLoading: errorsQuery.isLoading,
    syncing,
    statsLoading: statsQuery.isLoading,
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
