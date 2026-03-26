import type { LogsPageState } from "@/stores/logs/page-store";

export const selectLogsFilters = (state: LogsPageState) => state.filters;
export const selectLogsPage = (state: LogsPageState) => state.page;
export const selectLogsPageSize = (state: LogsPageState) => state.pageSize;

export const selectSelectedIds = (state: LogsPageState) => state.selectedIds;
export const selectSelectedLog = (state: LogsPageState) => state.selectedLog;

export const selectDetailDialogOpen = (state: LogsPageState) => state.detailDialogOpen;
export const selectDeleteDialogOpen = (state: LogsPageState) => state.deleteDialogOpen;
export const selectBatchDeleteDialogOpen = (state: LogsPageState) => state.batchDeleteDialogOpen;
export const selectClearAllDialogOpen = (state: LogsPageState) => state.clearAllDialogOpen;
export const selectClearFilteredDialogOpen = (state: LogsPageState) => state.clearFilteredDialogOpen;
export const selectLogToDelete = (state: LogsPageState) => state.logToDelete;

export const selectSetFilter = (state: LogsPageState) => state.setFilter;
export const selectSetPage = (state: LogsPageState) => state.setPage;
export const selectSetPageSize = (state: LogsPageState) => state.setPageSize;

export const selectSetSelectedIds = (state: LogsPageState) => state.setSelectedIds;
export const selectClearSelection = (state: LogsPageState) => state.clearSelection;

export const selectOpenDetailDialog = (state: LogsPageState) => state.openDetailDialog;
export const selectSetDetailDialogOpen = (state: LogsPageState) => state.setDetailDialogOpen;

export const selectOpenDeleteDialog = (state: LogsPageState) => state.openDeleteDialog;
export const selectSetDeleteDialogOpen = (state: LogsPageState) => state.setDeleteDialogOpen;

export const selectSetBatchDeleteDialogOpen = (state: LogsPageState) => state.setBatchDeleteDialogOpen;
export const selectSetClearAllDialogOpen = (state: LogsPageState) => state.setClearAllDialogOpen;
export const selectSetClearFilteredDialogOpen = (state: LogsPageState) => state.setClearFilteredDialogOpen;

export const selectResetLogsTransient = (state: LogsPageState) => state.resetTransient;

