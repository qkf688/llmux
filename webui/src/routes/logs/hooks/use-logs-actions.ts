import { useNavigate } from "react-router-dom";
import { getLogDetail, type ChatLog } from "@/lib/api";
import { toast } from "sonner";
import type { LogsFilters } from "../types";
import { exportChatLog, type ChatLogExportSections } from "../utils/export-log";
import { toApiLogsFilters, type LogsPageState } from "@/stores/logs";
import { toErrorMessage } from "@/lib/errors";
import {
  useDeleteLog,
  useBatchDeleteLogs,
  useClearAllLogs,
  useClearFilteredLogs,
} from "@/hooks/api/use-logs";

const needsLogDetail = (log: ChatLog) =>
  log.RequestHeaders === undefined &&
  log.RequestBody === undefined &&
  log.ResponseHeaders === undefined &&
  log.ResponseBody === undefined &&
  log.RawResponseBody === undefined;

const shouldIncludeRequestResponse = (sections: ChatLogExportSections) => sections.request || sections.response;

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
}: UseLogsActionsInput) {
  const navigate = useNavigate();
  const deleteLogMutation = useDeleteLog();
  const batchDeleteMutation = useBatchDeleteLogs();
  const clearAllMutation = useClearAllLogs();
  const clearFilteredMutation = useClearFilteredLogs();

  const canViewChatIO = (log: ChatLog) => log.Status === "success" && Boolean(log.ChatIO);

  const handleViewChatIO = (log: ChatLog) => {
    if (!canViewChatIO(log)) {
      return;
    }
    navigate(`/logs/${log.ID}/chat-io`);
  };

  const handleExportLog = async (log: ChatLog, sections: ChatLogExportSections) => {
    try {
      const exportLog =
        shouldIncludeRequestResponse(sections) && needsLogDetail(log) ? await getLogDetail(log.ID) : log;
      exportChatLog(exportLog, sections);
      toast.success("导出成功");
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`导出失败: ${message}`);
      throw error;
    }
  };

  const confirmDeleteLog = async () => {
    if (logToDelete === null) {
      return;
    }

    try {
      await deleteLogMutation.mutateAsync(logToDelete);
      toast.success("日志已删除");

      const nextSelected = new Set(selectedIds);
      nextSelected.delete(logToDelete);
      setSelectedIds(nextSelected);
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`删除失败: ${message}`);
    } finally {
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
      const result = await batchDeleteMutation.mutateAsync(Array.from(selectedIds));
      toast.success(`已删除 ${result.deleted} 条日志`);
      clearSelection();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`批量删除失败: ${message}`);
    } finally {
      setBatchDeleteDialogOpen(false);
    }
  };

  const confirmClearAllLogs = async () => {
    try {
      const result = await clearAllMutation.mutateAsync();
      toast.success(`已清空 ${result.deleted} 条日志`);
      clearSelection();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`清空日志失败: ${message}`);
    } finally {
      setClearAllDialogOpen(false);
    }
  };

  const confirmClearFilteredLogs = async () => {
    try {
      const result = await clearFilteredMutation.mutateAsync(toApiLogsFilters(filters));
      toast.success(`已清空筛选结果 ${result.deleted} 条日志`);
      clearSelection();

      if (page !== 1) {
        setPage(1);
      }
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`清空筛选结果失败: ${message}`);
    } finally {
      setClearFilteredDialogOpen(false);
    }
  };

  const isDeleting = deleteLogMutation.isPending || batchDeleteMutation.isPending;
  const isClearingAll = clearAllMutation.isPending;
  const isClearingFiltered = clearFilteredMutation.isPending;

  return {
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
  };
}
