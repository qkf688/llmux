import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  batchDeleteLogs,
  clearAllLogs,
  clearFilteredLogs,
  deleteLog,
  getLogs,
  getModels,
  getProviderTemplates,
  getProviders,
  getUserAgents,
  type ChatLog,
  type Model,
  type Provider,
} from "@/lib/api";
import { toast } from "sonner";
import { DEFAULT_LOGS_FILTERS, type LogsFilters } from "../types";
import { exportRequestResponse } from "../utils/export-log";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

const toApiFilters = (filters: LogsFilters) => ({
  providerName: filters.providerName === "all" ? undefined : filters.providerName,
  name: filters.model === "all" ? undefined : filters.model,
  status: filters.status === "all" ? undefined : filters.status,
  style: filters.style === "all" ? undefined : filters.style,
  userAgent: filters.userAgent === "all" ? undefined : filters.userAgent,
});

export function useLogsPage() {
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [logs, setLogs] = useState<ChatLog[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [userAgents, setUserAgents] = useState<string[]>([]);
  const [availableStyles, setAvailableStyles] = useState<string[]>([]);

  const [filters, setFilters] = useState<LogsFilters>({ ...DEFAULT_LOGS_FILTERS });
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [selectedLog, setSelectedLog] = useState<ChatLog | null>(null);

  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false);
  const [clearAllDialogOpen, setClearAllDialogOpen] = useState(false);
  const [clearFilteredDialogOpen, setClearFilteredDialogOpen] = useState(false);

  const [logToDelete, setLogToDelete] = useState<number | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isClearingAll, setIsClearingAll] = useState(false);
  const [isClearingFiltered, setIsClearingFiltered] = useState(false);

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
  }, []);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    try {
      const result = await getLogs(page, pageSize, toApiFilters(filters));
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
  }, [filters, page, pageSize]);

  useEffect(() => {
    void fetchFilterOptions();
  }, [fetchFilterOptions]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

  useEffect(() => {
    setSelectedIds(new Set());
  }, [page, pageSize, filters]);

  const handleFilterChange = (key: keyof LogsFilters, value: string) => {
    if (filters[key] === value) {
      return;
    }

    setFilters((previous) => ({ ...previous, [key]: value }));
    setPage(1);
  };

  const handlePageChange = (newPage: number) => {
    if (newPage >= 1 && newPage <= pages) {
      setPage(newPage);
    }
  };

  const handlePageSizeChange = (size: number) => {
    if (size === pageSize) {
      return;
    }
    setPage(1);
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
    setSelectedIds(new Set());
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  };

  const openDetailDialog = (log: ChatLog) => {
    setSelectedLog(log);
    setDetailDialogOpen(true);
  };

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
    if (!open) {
      setSelectedLog(null);
    }
  };

  const canViewChatIO = (log: ChatLog) => log.Status === "success" && Boolean(log.ChatIO);

  const handleViewChatIO = (log: ChatLog) => {
    if (!canViewChatIO(log)) {
      return;
    }
    navigate(`/logs/${log.ID}/chat-io`);
  };

  const handleExportRequestResponse = (log: ChatLog) => {
    try {
      exportRequestResponse(log);
      toast.success("导出成功");
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`导出失败: ${message}`);
    }
  };

  const openDeleteDialog = (id: number) => {
    setLogToDelete(id);
    setDeleteDialogOpen(true);
  };

  const handleDeleteDialogOpenChange = (open: boolean) => {
    setDeleteDialogOpen(open);
    if (!open) {
      setLogToDelete(null);
    }
  };

  const confirmDeleteLog = async () => {
    if (logToDelete === null) {
      return;
    }

    try {
      setIsDeleting(true);
      await deleteLog(logToDelete);
      toast.success("日志已删除");

      setSelectedIds((previous) => {
        const next = new Set(previous);
        next.delete(logToDelete);
        return next;
      });

      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    } finally {
      setIsDeleting(false);
      setDeleteDialogOpen(false);
      setLogToDelete(null);
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
      setSelectedIds(new Set());
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
      setSelectedIds(new Set());
      await fetchLogs();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`清空日志失败: ${message}`);
    } finally {
      setIsClearingAll(false);
      setClearAllDialogOpen(false);
    }
  };

  const canClearFiltered = Object.values(filters).some((value) => value !== "all");

  const filtersSummary = useMemo(() => {
    const parts: string[] = [];
    if (filters.status !== "all") {
      const label = filters.status === "success" ? "成功" : filters.status === "error" ? "错误" : filters.status;
      parts.push(`状态=${label}`);
    }
    if (filters.style !== "all") {
      parts.push(`类型=${filters.style}`);
    }
    if (filters.model !== "all") {
      parts.push(`模型=${filters.model}`);
    }
    if (filters.providerName !== "all") {
      parts.push(`提供商=${filters.providerName}`);
    }
    if (filters.userAgent !== "all") {
      const ua = filters.userAgent.length > 60 ? `${filters.userAgent.slice(0, 60)}...` : filters.userAgent;
      parts.push(`UA=${ua}`);
    }
    return parts.join("，");
  }, [filters.model, filters.providerName, filters.status, filters.style, filters.userAgent]);

  const confirmClearFilteredLogs = async () => {
    try {
      setIsClearingFiltered(true);
      const result = await clearFilteredLogs(toApiFilters(filters));
      toast.success(`已清空筛选结果 ${result.deleted} 条日志`);
      setSelectedIds(new Set());

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
