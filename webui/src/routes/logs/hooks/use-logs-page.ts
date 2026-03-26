import { useCallback, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  batchDeleteLogs,
  clearAllLogs,
  clearFilteredLogs,
  deleteLog,
  getLogDetail,
  getLogs,
  getModels,
  getProviderTemplates,
  getProviders,
  getUserAgents,
  type ChatLog,
} from "@/lib/api";
import { toast } from "sonner";
import type { LogsFilters } from "../types";
import { exportRequestResponse } from "../utils/export-log";
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
  toApiLogsFilters,
  useLogsPageStore,
} from "@/stores/logs";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

const needsLogDetail = (log: ChatLog) =>
  log.RequestHeaders === undefined &&
  log.RequestBody === undefined &&
  log.ResponseHeaders === undefined &&
  log.ResponseBody === undefined &&
  log.RawResponseBody === undefined;

export function useLogsPage() {
  const navigate = useNavigate();

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

  const fetchFilterOptions = useCallback(async () => {
    const [providerResult, modelResult, userAgentResult, templateResult] = await Promise.allSettled([
      getProviders(),
      getModels(),
      getUserAgents(),
      getProviderTemplates(),
    ]);

    if (providerResult.status === "fulfilled") {
      setProviders(providerResult.value);
    } else {
      console.error("Error fetching providers:", providerResult.reason);
    }

    if (modelResult.status === "fulfilled") {
      setModels(modelResult.value);
    } else {
      console.error("Error fetching models:", modelResult.reason);
    }

    if (userAgentResult.status === "fulfilled") {
      setUserAgents(userAgentResult.value);
    } else {
      console.error("Error fetching user agents:", userAgentResult.reason);
    }

    if (templateResult.status === "fulfilled") {
      const styleTypes = Array.from(
        new Set(templateResult.value.map((template) => template.type).filter(Boolean))
      );
      setAvailableStyles(styleTypes);
    } else {
      console.error("Error fetching provider templates:", templateResult.reason);
    }
  }, [setAvailableStyles, setModels, setProviders, setUserAgents]);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    try {
      const result = await getLogs(page, pageSize, toApiLogsFilters(filters));
      setLogs(result.data);
      setTotal(result.total);
      setPages(result.pages);
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`获取日志失败: ${message}`);
      console.error("Error fetching logs:", error);
    } finally {
      setLoading(false);
    }
  }, [filters, page, pageSize, setLoading, setLogs, setPages, setTotal]);

  useEffect(() => {
    void fetchFilterOptions();
  }, [fetchFilterOptions]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

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

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(logs.map((log) => log.ID)));
      return;
    }
    clearSelection();
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    const next = new Set(selectedIds);
    if (checked) {
      next.add(id);
    } else {
      next.delete(id);
    }
    setSelectedIds(next);
  };

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
  };

  const canViewChatIO = (log: ChatLog) => log.Status === "success" && Boolean(log.ChatIO);

  const handleViewChatIO = (log: ChatLog) => {
    if (!canViewChatIO(log)) {
      return;
    }
    navigate(`/logs/${log.ID}/chat-io`);
  };

  const handleExportRequestResponse = (log: ChatLog) => {
    void (async () => {
      try {
        const exportLog = needsLogDetail(log) ? await getLogDetail(log.ID) : log;
        exportRequestResponse(exportLog);
        toast.success("导出成功");
      } catch (error) {
        const message = toErrorMessage(error);
        toast.error(`导出失败: ${message}`);
      }
    })();
  };

  const handleDeleteDialogOpenChange = (open: boolean) => {
    setDeleteDialogOpen(open);
  };

  const confirmDeleteLog = async () => {
    if (logToDelete === null) {
      return;
    }

    try {
      setIsDeleting(true);
      await deleteLog(logToDelete);
      toast.success("日志已删除");

      const nextSelected = new Set(selectedIds);
      nextSelected.delete(logToDelete);
      setSelectedIds(nextSelected);

      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    } finally {
      setIsDeleting(false);
      setDeleteDialogOpen(false);
    }
  };

  const openBatchDeleteDialog = () => {
    if (selectedIds.size === 0) {
      return;
    }
    setBatchDeleteDialogOpen(true);
  };

  const confirmBatchDelete = async () => {
    if (selectedIds.size === 0) {
      return;
    }

    try {
      setIsDeleting(true);
      const result = await batchDeleteLogs(Array.from(selectedIds));
      toast.success(`已删除 ${result.deleted} 条日志`);
      clearSelection();
      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`批量删除失败: ${message}`);
    } finally {
      setIsDeleting(false);
      setBatchDeleteDialogOpen(false);
    }
  };

  const confirmClearAllLogs = async () => {
    try {
      setIsClearingAll(true);
      const result = await clearAllLogs();
      toast.success(`已清空 ${result.deleted} 条日志`);
      clearSelection();
      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`清空日志失败: ${message}`);
    } finally {
      setIsClearingAll(false);
      setClearAllDialogOpen(false);
    }
  };

  const canClearFiltered = hasActiveLogsFilters(filters);

  const filtersSummary = buildLogsFiltersSummary(filters);

  const confirmClearFilteredLogs = async () => {
    try {
      setIsClearingFiltered(true);
      const result = await clearFilteredLogs(toApiLogsFilters(filters));
      toast.success(`已清空筛选结果 ${result.deleted} 条日志`);
      clearSelection();

      if (page !== 1) {
        setPage(1);
        return;
      }
      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`清空筛选结果失败: ${message}`);
    } finally {
      setIsClearingFiltered(false);
      setClearFilteredDialogOpen(false);
    }
  };

  const selectedCount = selectedIds.size;
  const isAllSelected = logs.length > 0 && selectedCount === logs.length;
  const isSomeSelected = selectedCount > 0 && selectedCount < logs.length;

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
    handleExportRequestResponse,
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
