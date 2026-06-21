import type { HealthCheckBatchState } from "@/stores/health-check-logs/batch-store";
import type { HealthCheckLogsPageState } from "@/stores/health-check-logs/page-store";

export const selectHealthCheckBatchId = (state: HealthCheckBatchState) => state.batchId;
export const selectHealthCheckCompleted = (state: HealthCheckBatchState) => state.completed;
export const selectSetHealthCheckBatchState = (state: HealthCheckBatchState) => state.setBatchState;
export const selectMarkHealthCheckCompleted = (state: HealthCheckBatchState) => state.markCompleted;
export const selectClearHealthCheckBatchState = (state: HealthCheckBatchState) => state.clearBatchState;

export const selectHealthCheckLogsFilters = (state: HealthCheckLogsPageState) => state.filters;
export const selectHealthCheckLogsPage = (state: HealthCheckLogsPageState) => state.page;
export const selectHealthCheckLogsPageSize = (state: HealthCheckLogsPageState) => state.pageSize;

export const selectHealthCheckLogsDetailLog = (state: HealthCheckLogsPageState) => state.detailLog;
export const selectHealthCheckLogsDetailDialogOpen = (state: HealthCheckLogsPageState) => state.detailDialogOpen;
export const selectHealthCheckLogsClearDialogOpen = (state: HealthCheckLogsPageState) => state.clearDialogOpen;

export const selectHealthCheckLogsResultDialogOpen = (state: HealthCheckLogsPageState) => state.resultDialogOpen;
export const selectHealthCheckLogsCurrentBatchId = (state: HealthCheckLogsPageState) => state.currentBatchId;
export const selectHealthCheckLogsBackgroundBatchId = (state: HealthCheckLogsPageState) => state.backgroundBatchId;
export const selectHealthCheckLogsBackgroundCheckComplete = (state: HealthCheckLogsPageState) =>
  state.backgroundCheckComplete;

export const selectSetHealthCheckLogsFilter = (state: HealthCheckLogsPageState) => state.setFilter;
export const selectSetHealthCheckLogsPage = (state: HealthCheckLogsPageState) => state.setPage;
export const selectSetHealthCheckLogsPageSize = (state: HealthCheckLogsPageState) => state.setPageSize;

export const selectOpenHealthCheckLogsDetailDialog = (state: HealthCheckLogsPageState) => state.openDetailDialog;
export const selectSetHealthCheckLogsDetailDialogOpen = (state: HealthCheckLogsPageState) => state.setDetailDialogOpen;

export const selectSetHealthCheckLogsClearDialogOpen = (state: HealthCheckLogsPageState) => state.setClearDialogOpen;

export const selectSetHealthCheckLogsResultDialogOpen = (state: HealthCheckLogsPageState) => state.setResultDialogOpen;
export const selectSetHealthCheckLogsCurrentBatchId = (state: HealthCheckLogsPageState) => state.setCurrentBatchId;
export const selectSetHealthCheckLogsBackgroundBatchId = (state: HealthCheckLogsPageState) => state.setBackgroundBatchId;
export const selectSetHealthCheckLogsBackgroundCheckComplete = (state: HealthCheckLogsPageState) =>
  state.setBackgroundCheckComplete;

export const selectResetHealthCheckLogsTransient = (state: HealthCheckLogsPageState) => state.resetTransient;
