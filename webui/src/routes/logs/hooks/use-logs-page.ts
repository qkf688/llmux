import { useEffect } from "react";
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
  selectLogs,
  selectLogsAvailableStyles,
  selectLogsFilters,
  selectLogsIsClearingAll,
  selectLogsIsClearingFiltered,
  selectLogsIsDeleting,
  selectLogsLoading,
  selectLogsPage,
  selectLogsPages,
  selectLogsPageSize,
  selectLogsProviders,
  selectLogsModels,
  selectLogsTotal,
  selectLogsUserAgents,
  selectOpenDeleteDialog,
  selectOpenDetailDialog,
  selectResetLogsTransient,
  selectSelectedIds,
  selectSelectedLog,
  selectSetBatchDeleteDialogOpen,
  selectSetClearAllDialogOpen,
  selectSetClearFilteredDialogOpen,
  selectSetDeleteDialogOpen,
  selectSetDetailDialogOpen,
  selectSetFilter,
  selectSetLogs,
  selectSetLogsAvailableStyles,
  selectSetLogsIsClearingAll,
  selectSetLogsIsClearingFiltered,
  selectSetLogsIsDeleting,
  selectSetLogsLoading,
  selectSetLogsModels,
  selectSetLogsPages,
  selectSetLogsProviders,
  selectSetLogsTotal,
  selectSetLogsUserAgents,
  selectSetPage,
  selectSetPageSize,
  selectSetSelectedIds,
  useLogsPageStore,
} from "@/stores/logs";
import { useLogsActions } from "./use-logs-actions";
import { useLogsFetchers } from "./use-logs-fetchers";
import { useLogsSelection } from "./use-logs-selection";

export function useLogsPage() {
  const loading = useLogsPageStore(selectLogsLoading);
  const logs = useLogsPageStore(selectLogs);
  const providers = useLogsPageStore(selectLogsProviders);
  const models = useLogsPageStore(selectLogsModels);
  const userAgents = useLogsPageStore(selectLogsUserAgents);
  const availableStyles = useLogsPageStore(selectLogsAvailableStyles);
  const total = useLogsPageStore(selectLogsTotal);
  const pages = useLogsPageStore(selectLogsPages);

  const filters = useLogsPageStore(selectLogsFilters);
  const page = useLogsPageStore(selectLogsPage);
  const pageSize = useLogsPageStore(selectLogsPageSize);
  const setLoading = useLogsPageStore(selectSetLogsLoading);
  const setLogs = useLogsPageStore(selectSetLogs);
  const setProviders = useLogsPageStore(selectSetLogsProviders);
  const setModels = useLogsPageStore(selectSetLogsModels);
  const setUserAgents = useLogsPageStore(selectSetLogsUserAgents);
  const setAvailableStyles = useLogsPageStore(selectSetLogsAvailableStyles);
  const setFilter = useLogsPageStore(selectSetFilter);
  const setPage = useLogsPageStore(selectSetPage);
  const setPageSize = useLogsPageStore(selectSetPageSize);
  const setTotal = useLogsPageStore(selectSetLogsTotal);
  const setPages = useLogsPageStore(selectSetLogsPages);

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
  const resetTransient = useLogsPageStore(selectResetLogsTransient);
  const isDeleting = useLogsPageStore(selectLogsIsDeleting);
  const isClearingAll = useLogsPageStore(selectLogsIsClearingAll);
  const isClearingFiltered = useLogsPageStore(selectLogsIsClearingFiltered);
  const setIsDeleting = useLogsPageStore(selectSetLogsIsDeleting);
  const setIsClearingAll = useLogsPageStore(selectSetLogsIsClearingAll);
  const setIsClearingFiltered = useLogsPageStore(selectSetLogsIsClearingFiltered);

  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  const { fetchLogs } = useLogsFetchers({
    filters,
    page,
    pageSize,
    setLoading,
    setLogs,
    setTotal,
    setPages,
    setProviders,
    setModels,
    setUserAgents,
    setAvailableStyles,
  });

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

  const refreshLogs = () => {
    void fetchLogs();
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
    setIsDeleting,
    setIsClearingAll,
    setIsClearingFiltered,
    fetchLogs,
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
