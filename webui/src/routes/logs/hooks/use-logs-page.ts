import { useCallback, useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { LogsFilters } from "../types";
import {
  buildLogsFiltersSummary,
  hasActiveLogsFilters,
  selectBatchDeleteDialogOpen,
  selectClearAllDialogOpen,
  selectClearFilteredDialogOpen,
  selectClearSelection,
  selectDeleteDialogOpen,
  selectDetailDialogOpen,
  selectLogToDelete,
  selectLogsFilters,
  selectLogsPage,
  selectLogsPageSize,
  selectOpenDeleteDialog,
  selectOpenDetailDialog,
  selectSelectedIds,
  selectSelectedLog,
  selectSetBatchDeleteDialogOpen,
  selectSetClearAllDialogOpen,
  selectSetClearFilteredDialogOpen,
  selectSetDeleteDialogOpen,
  selectSetDetailDialogOpen,
  selectSetFilter,
  selectSetPage,
  selectSetPageSize,
  selectSetSelectedIds,
  useLogsPageStore,
} from "@/stores/logs";
import { toApiLogsFilters } from "@/stores/logs";
import { useLogsQuery, useUserAgents, logsKeys } from "@/hooks/api/use-logs";
import { useModels } from "@/hooks/api/use-models";
import { useProviders, useProviderTemplates } from "@/hooks/api/use-providers";
import { useLogsActions } from "./use-logs-actions";
import { useLogsSelection } from "./use-logs-selection";

export function useLogsPage() {
  const filters = useLogsPageStore(selectLogsFilters);
  const page = useLogsPageStore(selectLogsPage);
  const pageSize = useLogsPageStore(selectLogsPageSize);
  const setFilter = useLogsPageStore(selectSetFilter);
  const setPage = useLogsPageStore(selectSetPage);
  const setPageSize = useLogsPageStore(selectSetPageSize);

  const selectedIds = useLogsPageStore(selectSelectedIds);
  const setSelectedIds = useLogsPageStore(selectSetSelectedIds);
  const clearSelection = useLogsPageStore(selectClearSelection);

  const selectedLog = useLogsPageStore(selectSelectedLog);
  const detailDialogOpen = useLogsPageStore(selectDetailDialogOpen);
  const deleteDialogOpen = useLogsPageStore(selectDeleteDialogOpen);
  const batchDeleteDialogOpen = useLogsPageStore(selectBatchDeleteDialogOpen);
  const clearAllDialogOpen = useLogsPageStore(selectClearAllDialogOpen);
  const clearFilteredDialogOpen = useLogsPageStore(selectClearFilteredDialogOpen);

  const openDetailDialog = useLogsPageStore(selectOpenDetailDialog);
  const setDetailDialogOpen = useLogsPageStore(selectSetDetailDialogOpen);

  const openDeleteDialog = useLogsPageStore(selectOpenDeleteDialog);
  const setDeleteDialogOpen = useLogsPageStore(selectSetDeleteDialogOpen);

  const setBatchDeleteDialogOpen = useLogsPageStore(selectSetBatchDeleteDialogOpen);
  const setClearAllDialogOpen = useLogsPageStore(selectSetClearAllDialogOpen);
  const setClearFilteredDialogOpen = useLogsPageStore(selectSetClearFilteredDialogOpen);

  const logToDelete = useLogsPageStore(selectLogToDelete);

  const queryClient = useQueryClient();
  const refreshLogs = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: logsKeys.all });
  }, [queryClient]);

  const apiFilters = useMemo(() => toApiLogsFilters(filters), [filters]);
  const { data: logsResponse, isLoading: loading } = useLogsQuery(page, pageSize, apiFilters);
  const logs = logsResponse?.data ?? [];
  const total = logsResponse?.total ?? 0;
  const pages = logsResponse?.pages ?? 0;

  const { data: providers = [] } = useProviders();
  const { data: models = [] } = useModels();
  const { data: userAgents = [] } = useUserAgents();
  const { data: templates = [] } = useProviderTemplates();
  const availableStyles = useMemo(
    () => Array.from(new Set(templates.map((t) => t.type).filter(Boolean))),
    [templates],
  );

  const handleFilterChange = (key: keyof LogsFilters, value: string) => {
    setFilter(key, value);
  };

  const handlePageChange = (newPage: number) => {
    if (newPage >= 1 && newPage <= pages) {
      setPage(newPage);
    }
  };

  const handlePageSizeChange = (size: number) => {
    setPageSize(size);
  };

  const { selectedCount, isAllSelected, isSomeSelected, handleSelectAll, handleSelectOne } = useLogsSelection({
    logs,
    selectedIds,
    setSelectedIds,
    clearSelection,
  });

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
  };

  const handleDeleteDialogOpenChange = (open: boolean) => {
    setDeleteDialogOpen(open);
  };

  const canClearFiltered = hasActiveLogsFilters(filters);
  const filtersSummary = buildLogsFiltersSummary(filters);

  const {
    canViewChatIO,
    handleViewChatIO,
    handleExportLog,
    confirmDeleteLog,
    openBatchDeleteDialog,
    confirmBatchDelete,
    confirmClearAllLogs,
    confirmClearFilteredLogs,
    isDeleting,
    isClearingAll,
    isClearingFiltered,
  } = useLogsActions({
    filters,
    page,
    selectedIds,
    logToDelete,
    setSelectedIds,
    clearSelection,
    setPage,
    setDeleteDialogOpen,
    setBatchDeleteDialogOpen,
    setClearAllDialogOpen,
    setClearFilteredDialogOpen,
  });

  const hasLogs = logs.length > 0;

  return {
    loading,
    logs,
    hasLogs,
    providers,
    models,
    userAgents,
    availableStyles,
    filters,
    filtersSummary,
    page,
    pageSize,
    total,
    pages,
    selectedIds,
    selectedCount,
    isAllSelected,
    isSomeSelected,
    selectedLog,
    detailDialogOpen,
    deleteDialogOpen,
    batchDeleteDialogOpen,
    clearAllDialogOpen,
    clearFilteredDialogOpen,
    isDeleting,
    isClearingAll,
    isClearingFiltered,
    canClearFiltered,
    handleFilterChange,
    handlePageChange,
    handlePageSizeChange,
    refreshLogs,
    handleSelectAll,
    handleSelectOne,
    openDetailDialog,
    handleDetailDialogOpenChange,
    canViewChatIO,
    handleViewChatIO,
    handleExportLog,
    openDeleteDialog,
    handleDeleteDialogOpenChange,
    confirmDeleteLog,
    openBatchDeleteDialog,
    confirmBatchDelete,
    confirmClearAllLogs,
    confirmClearFilteredLogs,
    setBatchDeleteDialogOpen,
    setClearAllDialogOpen,
    setClearFilteredDialogOpen,
  };
}
