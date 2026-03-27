import type { ModelSyncLogsPageState } from "@/stores/model-sync-logs/page-store";

export const selectModelSyncLogsActiveTab = (state: ModelSyncLogsPageState) => state.activeTab;
export const selectModelSyncLogsLogs = (state: ModelSyncLogsPageState) => state.logs;
export const selectModelSyncLogsRecentModels = (state: ModelSyncLogsPageState) => state.recentModels;
export const selectModelSyncLogsRecentErrors = (state: ModelSyncLogsPageState) => state.recentErrors;
export const selectModelSyncLogsSyncTime = (state: ModelSyncLogsPageState) => state.syncTime;
export const selectModelSyncLogsStats = (state: ModelSyncLogsPageState) => state.stats;
export const selectModelSyncLogsProvidersById = (state: ModelSyncLogsPageState) => state.providersById;

export const selectModelSyncLogsLoading = (state: ModelSyncLogsPageState) => state.loading;
export const selectModelSyncLogsRecentLoading = (state: ModelSyncLogsPageState) => state.recentLoading;
export const selectModelSyncLogsErrorsLoading = (state: ModelSyncLogsPageState) => state.errorsLoading;
export const selectModelSyncLogsSyncing = (state: ModelSyncLogsPageState) => state.syncing;
export const selectModelSyncLogsStatsLoading = (state: ModelSyncLogsPageState) => state.statsLoading;

export const selectModelSyncLogsSelectedLogs = (state: ModelSyncLogsPageState) => state.selectedLogs;
export const selectModelSyncLogsSelectedErrorProviders = (state: ModelSyncLogsPageState) => state.selectedErrorProviders;
export const selectModelSyncLogsPage = (state: ModelSyncLogsPageState) => state.page;
export const selectModelSyncLogsTotalPages = (state: ModelSyncLogsPageState) => state.totalPages;
export const selectModelSyncLogsShowUnchanged = (state: ModelSyncLogsPageState) => state.showUnchanged;
export const selectModelSyncLogsTogglingProviderIds = (state: ModelSyncLogsPageState) => state.togglingProviderIds;

export const selectModelSyncLogsDetailLog = (state: ModelSyncLogsPageState) => state.detailLog;
export const selectModelSyncLogsClearDialogOpen = (state: ModelSyncLogsPageState) => state.clearDialogOpen;
export const selectModelSyncLogsClearingErrors = (state: ModelSyncLogsPageState) => state.clearingErrors;

export const selectSetModelSyncLogsActiveTab = (state: ModelSyncLogsPageState) => state.setActiveTab;
export const selectSetModelSyncLogsLogs = (state: ModelSyncLogsPageState) => state.setLogs;
export const selectSetModelSyncLogsRecentModels = (state: ModelSyncLogsPageState) => state.setRecentModels;
export const selectSetModelSyncLogsRecentErrors = (state: ModelSyncLogsPageState) => state.setRecentErrors;
export const selectSetModelSyncLogsSyncTime = (state: ModelSyncLogsPageState) => state.setSyncTime;
export const selectSetModelSyncLogsStats = (state: ModelSyncLogsPageState) => state.setStats;
export const selectSetModelSyncLogsProvidersById = (state: ModelSyncLogsPageState) => state.setProvidersById;

export const selectSetModelSyncLogsLoading = (state: ModelSyncLogsPageState) => state.setLoading;
export const selectSetModelSyncLogsRecentLoading = (state: ModelSyncLogsPageState) => state.setRecentLoading;
export const selectSetModelSyncLogsErrorsLoading = (state: ModelSyncLogsPageState) => state.setErrorsLoading;
export const selectSetModelSyncLogsSyncing = (state: ModelSyncLogsPageState) => state.setSyncing;
export const selectSetModelSyncLogsStatsLoading = (state: ModelSyncLogsPageState) => state.setStatsLoading;

export const selectSetModelSyncLogsSelectedLogs = (state: ModelSyncLogsPageState) => state.setSelectedLogs;
export const selectSetModelSyncLogsSelectedErrorProviders = (state: ModelSyncLogsPageState) => state.setSelectedErrorProviders;
export const selectSetModelSyncLogsPage = (state: ModelSyncLogsPageState) => state.setPage;
export const selectSetModelSyncLogsTotalPages = (state: ModelSyncLogsPageState) => state.setTotalPages;
export const selectSetModelSyncLogsShowUnchanged = (state: ModelSyncLogsPageState) => state.setShowUnchanged;
export const selectSetModelSyncLogsTogglingProviderIds = (state: ModelSyncLogsPageState) => state.setTogglingProviderIds;

export const selectSetModelSyncLogsDetailLog = (state: ModelSyncLogsPageState) => state.setDetailLog;
export const selectSetModelSyncLogsClearDialogOpen = (state: ModelSyncLogsPageState) => state.setClearDialogOpen;
export const selectSetModelSyncLogsClearingErrors = (state: ModelSyncLogsPageState) => state.setClearingErrors;

export const selectResetModelSyncLogsTransient = (state: ModelSyncLogsPageState) => state.resetTransient;

