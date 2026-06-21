import type { ModelSyncLogsPageState } from "@/stores/model-sync-logs/page-store";

export const selectModelSyncLogsActiveTab = (state: ModelSyncLogsPageState) => state.activeTab;
export const selectModelSyncLogsShowUnchanged = (state: ModelSyncLogsPageState) => state.showUnchanged;

export const selectModelSyncLogsSelectedLogs = (state: ModelSyncLogsPageState) => state.selectedLogs;
export const selectModelSyncLogsSelectedErrorProviders = (state: ModelSyncLogsPageState) => state.selectedErrorProviders;
export const selectModelSyncLogsPage = (state: ModelSyncLogsPageState) => state.page;
export const selectModelSyncLogsTogglingProviderIds = (state: ModelSyncLogsPageState) => state.togglingProviderIds;

export const selectModelSyncLogsDetailLog = (state: ModelSyncLogsPageState) => state.detailLog;
export const selectModelSyncLogsClearDialogOpen = (state: ModelSyncLogsPageState) => state.clearDialogOpen;

export const selectSetModelSyncLogsActiveTab = (state: ModelSyncLogsPageState) => state.setActiveTab;
export const selectSetModelSyncLogsShowUnchanged = (state: ModelSyncLogsPageState) => state.setShowUnchanged;

export const selectSetModelSyncLogsSelectedLogs = (state: ModelSyncLogsPageState) => state.setSelectedLogs;
export const selectSetModelSyncLogsSelectedErrorProviders = (state: ModelSyncLogsPageState) => state.setSelectedErrorProviders;
export const selectSetModelSyncLogsPage = (state: ModelSyncLogsPageState) => state.setPage;
export const selectSetModelSyncLogsTogglingProviderIds = (state: ModelSyncLogsPageState) => state.setTogglingProviderIds;

export const selectSetModelSyncLogsDetailLog = (state: ModelSyncLogsPageState) => state.setDetailLog;
export const selectSetModelSyncLogsClearDialogOpen = (state: ModelSyncLogsPageState) => state.setClearDialogOpen;

export const selectResetModelSyncLogsTransient = (state: ModelSyncLogsPageState) => state.resetTransient;
