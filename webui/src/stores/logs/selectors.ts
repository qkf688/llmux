import type { LogsPageState } from "@/stores/logs/page-store";

export const selectLogsLoading = (state: LogsPageState) => state.loading;
export const selectLogs = (state: LogsPageState) => state.logs;
export const selectLogsProviders = (state: LogsPageState) => state.providers;
export const selectLogsModels = (state: LogsPageState) => state.models;
export const selectLogsUserAgents = (state: LogsPageState) => state.userAgents;
export const selectLogsAvailableStyles = (state: LogsPageState) => state.availableStyles;
export const selectLogsFilters = (state: LogsPageState) => state.filters;
export const selectLogsPage = (state: LogsPageState) => state.page;
export const selectLogsPageSize = (state: LogsPageState) => state.pageSize;
export const selectLogsTotal = (state: LogsPageState) => state.total;
export const selectLogsPages = (state: LogsPageState) => state.pages;

export const selectSelectedIds = (state: LogsPageState) => state.selectedIds;
export const selectSelectedLog = (state: LogsPageState) => state.selectedLog;

export const selectDetailDialogOpen = (state: LogsPageState) => state.detailDialogOpen;
export const selectDeleteDialogOpen = (state: LogsPageState) => state.deleteDialogOpen;
export const selectBatchDeleteDialogOpen = (state: LogsPageState) => state.batchDeleteDialogOpen;
export const selectClearAllDialogOpen = (state: LogsPageState) => state.clearAllDialogOpen;
export const selectClearFilteredDialogOpen = (state: LogsPageState) => state.clearFilteredDialogOpen;
export const selectLogToDelete = (state: LogsPageState) => state.logToDelete;

export const selectLogsIsDeleting = (state: LogsPageState) => state.isDeleting;
export const selectLogsIsClearingAll = (state: LogsPageState) => state.isClearingAll;
export const selectLogsIsClearingFiltered = (state: LogsPageState) => state.isClearingFiltered;

export const selectSetLogsLoading = (state: LogsPageState) => state.setLoading;
export const selectSetLogs = (state: LogsPageState) => state.setLogs;
export const selectSetLogsProviders = (state: LogsPageState) => state.setProviders;
export const selectSetLogsModels = (state: LogsPageState) => state.setModels;
export const selectSetLogsUserAgents = (state: LogsPageState) => state.setUserAgents;
export const selectSetLogsAvailableStyles = (state: LogsPageState) => state.setAvailableStyles;
export const selectSetLogsTotal = (state: LogsPageState) => state.setTotal;
export const selectSetLogsPages = (state: LogsPageState) => state.setPages;

export const selectSetFilter = (state: LogsPageState) => state.setFilter;
export const selectSetPage = (state: LogsPageState) => state.setPage;
export const selectSetPageSize = (state: LogsPageState) => state.setPageSize;

export const selectSetSelectedIds = (state: LogsPageState) => state.setSelectedIds;
export const selectClearSelection = (state: LogsPageState) => state.clearSelection;

export const selectOpenDetailDialog = (state: LogsPageState) => state.openDetailDialog;
export const selectSetDetailDialogOpen = (state: LogsPageState) => state.setDetailDialogOpen;

export const selectOpenDeleteDialog = (state: LogsPageState) => state.openDeleteDialog;
export const selectSetDeleteDialogOpen = (state: LogsPageState) => state.setDeleteDialogOpen;

export const selectSetLogsIsDeleting = (state: LogsPageState) => state.setIsDeleting;
export const selectSetLogsIsClearingAll = (state: LogsPageState) => state.setIsClearingAll;
export const selectSetLogsIsClearingFiltered = (state: LogsPageState) => state.setIsClearingFiltered;

export const selectSetBatchDeleteDialogOpen = (state: LogsPageState) => state.setBatchDeleteDialogOpen;
export const selectSetClearAllDialogOpen = (state: LogsPageState) => state.setClearAllDialogOpen;
export const selectSetClearFilteredDialogOpen = (state: LogsPageState) => state.setClearFilteredDialogOpen;

export const selectResetLogsTransient = (state: LogsPageState) => state.resetTransient;
