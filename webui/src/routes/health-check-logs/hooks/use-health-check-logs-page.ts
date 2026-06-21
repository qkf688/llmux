import { useCallback, useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  getBatchHealthCheckStatus,
  runHealthCheckAll,
} from "@/lib/api";
import { toast } from "sonner";
import type { HealthCheckLogsFilters } from "../types";
import { toHealthCheckLogsApiFilters } from "../utils/filters";
import {
  clearStoredHealthCheckBatchState,
  markStoredHealthCheckBatchCompleted,
  readStoredHealthCheckBatchState,
  writeStoredHealthCheckBatchState,
} from "../utils/storage";
import {
  selectHealthCheckLogsBackgroundBatchId,
  selectHealthCheckLogsBackgroundCheckComplete,
  selectHealthCheckLogsClearDialogOpen,
  selectHealthCheckLogsDetailDialogOpen,
  selectHealthCheckLogsDetailLog,
  selectHealthCheckLogsFilters,
  selectHealthCheckLogsPage,
  selectHealthCheckLogsPageSize,
  selectHealthCheckLogsResultDialogOpen,
  selectHealthCheckLogsCurrentBatchId,
  selectOpenHealthCheckLogsDetailDialog,
  selectResetHealthCheckLogsTransient,
  selectSetHealthCheckLogsBackgroundBatchId,
  selectSetHealthCheckLogsBackgroundCheckComplete,
  selectSetHealthCheckLogsClearDialogOpen,
  selectSetHealthCheckLogsDetailDialogOpen,
  selectSetHealthCheckLogsFilter,
  selectSetHealthCheckLogsPage,
  selectSetHealthCheckLogsPageSize,
  selectSetHealthCheckLogsResultDialogOpen,
  selectSetHealthCheckLogsCurrentBatchId,
  useHealthCheckLogsPageStore,
} from "@/stores/health-check-logs";
import {
  healthCheckLogsKeys,
  useHealthCheckLogsQuery,
  useClearHealthCheckLogs,
  useProviders,
  useModels,
} from "@/hooks/api";
import { toErrorMessage } from "@/lib/errors";

export function useHealthCheckLogsPage() {
  const queryClient = useQueryClient();

  const filters = useHealthCheckLogsPageStore(selectHealthCheckLogsFilters);
  const page = useHealthCheckLogsPageStore(selectHealthCheckLogsPage);
  const pageSize = useHealthCheckLogsPageStore(selectHealthCheckLogsPageSize);
  const detailLog = useHealthCheckLogsPageStore(selectHealthCheckLogsDetailLog);
  const detailDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsDetailDialogOpen);
  const clearDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsClearDialogOpen);
  const resultDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsResultDialogOpen);
  const currentBatchId = useHealthCheckLogsPageStore(selectHealthCheckLogsCurrentBatchId);
  const backgroundBatchId = useHealthCheckLogsPageStore(selectHealthCheckLogsBackgroundBatchId);
  const backgroundCheckComplete = useHealthCheckLogsPageStore(selectHealthCheckLogsBackgroundCheckComplete);

  const setFilter = useHealthCheckLogsPageStore(selectSetHealthCheckLogsFilter);
  const setPage = useHealthCheckLogsPageStore(selectSetHealthCheckLogsPage);
  const setPageSize = useHealthCheckLogsPageStore(selectSetHealthCheckLogsPageSize);
  const openDetailDialog = useHealthCheckLogsPageStore(selectOpenHealthCheckLogsDetailDialog);
  const setDetailDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsDetailDialogOpen);
  const setClearDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsClearDialogOpen);
  const setResultDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsResultDialogOpen);
  const setCurrentBatchId = useHealthCheckLogsPageStore(selectSetHealthCheckLogsCurrentBatchId);
  const setBackgroundBatchId = useHealthCheckLogsPageStore(selectSetHealthCheckLogsBackgroundBatchId);
  const setBackgroundCheckComplete = useHealthCheckLogsPageStore(selectSetHealthCheckLogsBackgroundCheckComplete);
  const resetTransient = useHealthCheckLogsPageStore(selectResetHealthCheckLogsTransient);

  const apiFilters = toHealthCheckLogsApiFilters(filters);
  const logsResponse = useHealthCheckLogsQuery(page, pageSize, apiFilters);
  const { data: providers = [] } = useProviders();
  const { data: models = [] } = useModels();
  const clearMutation = useClearHealthCheckLogs();

  const logs = logsResponse.data?.data ?? [];
  const total = logsResponse.data?.total ?? 0;
  const pages = logsResponse.data?.pages ?? 0;
  const loading = logsResponse.isLoading;

  const pollingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const resultDialogOpenRef = useRef(resultDialogOpen);

  const stopBackgroundPolling = useCallback(() => {
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }
  }, []);

  useEffect(() => {
    resultDialogOpenRef.current = resultDialogOpen;
  }, [resultDialogOpen]);

  const refreshLogs = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: healthCheckLogsKeys.all });
  }, [queryClient]);

  const handleFilterChange = (key: keyof HealthCheckLogsFilters, value: string) => {
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

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
  };

  const confirmClearLogs = async () => {
    try {
      const result = await clearMutation.mutateAsync();
      toast.success(`已清空 ${result.deleted} 条健康检测日志`);
      setClearDialogOpen(false);
      setPage(1);
    } catch (error) {
      toast.error(`清空健康检测日志失败: ${toErrorMessage(error)}`);
    }
  };

  const startBackgroundPolling = useCallback(
    (batchId: string) => {
      if (pollingIntervalRef.current) {
        clearInterval(pollingIntervalRef.current);
      }

      pollingIntervalRef.current = setInterval(async () => {
        try {
          const status = await getBatchHealthCheckStatus(batchId);
          if (status.completed) {
            setBackgroundCheckComplete(true);
            markStoredHealthCheckBatchCompleted();
            stopBackgroundPolling();

            if (!resultDialogOpenRef.current) {
              void queryClient.invalidateQueries({ queryKey: healthCheckLogsKeys.all });
            }
          }
        } catch (error) {
          console.error("Failed to fetch background batch status:", error);
        }
      }, 3000);
    },
    [setBackgroundCheckComplete, stopBackgroundPolling, queryClient]
  );

  const handleRunHealthCheck = async () => {
    if (backgroundBatchId && !backgroundCheckComplete) {
      setResultDialogOpen(true);
      return;
    }

    try {
      const { batch_id } = await runHealthCheckAll();
      setCurrentBatchId(batch_id);
      setBackgroundBatchId(batch_id);
      setBackgroundCheckComplete(false);
      setResultDialogOpen(true);
      writeStoredHealthCheckBatchState(batch_id, false);
      startBackgroundPolling(batch_id);
    } catch (error) {
      toast.error(`启动健康检测失败: ${toErrorMessage(error)}`);
    }
  };

  const handleResultDialogOpenChange = (open: boolean) => {
    setResultDialogOpen(open);
    if (!open && backgroundCheckComplete) {
      setCurrentBatchId(null);
      setBackgroundBatchId(null);
      refreshLogs();
    }
  };

  const showProgressDialog = () => {
    setResultDialogOpen(true);
  };

  const handleDismissBanner = () => {
    setBackgroundBatchId(null);
    stopBackgroundPolling();
    clearStoredHealthCheckBatchState();
  };

  useEffect(
    () => () => {
      stopBackgroundPolling();
      resetTransient();
    },
    [resetTransient, stopBackgroundPolling]
  );

  useEffect(() => {
    const storedState = readStoredHealthCheckBatchState();
    if (!storedState.batchId) {
      return;
    }

    setBackgroundBatchId(storedState.batchId);
    setCurrentBatchId(storedState.batchId);
    setBackgroundCheckComplete(storedState.completed);

    if (!storedState.completed) {
      startBackgroundPolling(storedState.batchId);
    }
  }, [setBackgroundBatchId, setBackgroundCheckComplete, setCurrentBatchId, startBackgroundPolling]);

  return {
    loading,
    logs,
    hasLogs: logs.length > 0,
    providers,
    models,
    filters,
    page,
    pageSize,
    total,
    pages,
    detailLog,
    detailDialogOpen,
    clearDialogOpen,
    clearingLogs: clearMutation.isPending,
    resultDialogOpen,
    currentBatchId,
    backgroundBatchId,
    backgroundCheckComplete,
    runningBackgroundCheck: Boolean(backgroundBatchId) && !backgroundCheckComplete,
    handleFilterChange,
    handlePageChange,
    handlePageSizeChange,
    refreshLogs,
    openDetailDialog,
    handleDetailDialogOpenChange,
    confirmClearLogs,
    handleRunHealthCheck,
    handleResultDialogOpenChange,
    showProgressDialog,
    handleDismissBanner,
    setClearDialogOpen,
  };
}
