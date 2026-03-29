import { useNavigate } from "react-router-dom";
import {
  batchDeleteLogs,
  clearAllLogs,
  clearFilteredLogs,
  deleteLog,
  getLogDetail,
  type ChatLog,
} from "@/lib/api";
import { toast } from "sonner";
import type { LogsFilters } from "../types";
import { exportRequestResponse } from "../utils/export-log";
import { toApiLogsFilters, type LogsPageState } from "@/stores/logs";
import { toErrorMessage } from "@/lib/errors";

const needsLogDetail = (log: ChatLog) =>
  log.RequestHeaders === undefined &&
  log.RequestBody === undefined &&
  log.ResponseHeaders === undefined &&
  log.ResponseBody === undefined &&
  log.RawResponseBody === undefined;

type UseLogsActionsInput = {
  filters: LogsFilters;
  page: LogsPageState["page"];
  selectedIds: LogsPageState["selectedIds"];
  logToDelete: LogsPageState["logToDelete"];
  setSelectedIds: LogsPageState["setSelectedIds"];
  clearSelection: LogsPageState["clearSelection"];
  setPage: LogsPageState["setPage"];
  setDeleteDialogOpen: LogsPageState["setDeleteDialogOpen"];
  setBatchDeleteDialogOpen: LogsPageState["setBatchDeleteDialogOpen"];
  setClearAllDialogOpen: LogsPageState["setClearAllDialogOpen"];
  setClearFilteredDialogOpen: LogsPageState["setClearFilteredDialogOpen"];
  setIsDeleting: LogsPageState["setIsDeleting"];
  setIsClearingAll: LogsPageState["setIsClearingAll"];
  setIsClearingFiltered: LogsPageState["setIsClearingFiltered"];
  fetchLogs: () => Promise<void>;
};

export function useLogsActions({
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
}: UseLogsActionsInput) {
  const navigate = useNavigate();

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

  return {
    canViewChatIO,
    handleViewChatIO,
    handleExportRequestResponse,
    confirmDeleteLog,
    openBatchDeleteDialog,
    confirmBatchDelete,
    confirmClearAllLogs,
    confirmClearFilteredLogs,
  };
}
