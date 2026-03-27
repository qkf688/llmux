import { useCallback, useEffect, useRef } from "react";
import {
  clearHealthCheckLogs,
  getBatchHealthCheckStatus,
  getHealthCheckLogs,
  getModels,
  getProviders,
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
  selectHealthCheckLogsClearingLogs,
  selectHealthCheckLogsClearDialogOpen,
  selectHealthCheckLogsDetailDialogOpen,
  selectHealthCheckLogsDetailLog,
  selectHealthCheckLogsFilters,
  selectHealthCheckLogsLoading,
  selectHealthCheckLogsLogs,
  selectHealthCheckLogsModels,
  selectHealthCheckLogsPage,
  selectHealthCheckLogsPages,
  selectHealthCheckLogsPageSize,
  selectHealthCheckLogsProviders,
  selectHealthCheckLogsResultDialogOpen,
  selectHealthCheckLogsTotal,
  selectHealthCheckLogsCurrentBatchId,
  selectOpenHealthCheckLogsDetailDialog,
  selectResetHealthCheckLogsTransient,
  selectSetHealthCheckLogsBackgroundBatchId,
  selectSetHealthCheckLogsBackgroundCheckComplete,
  selectSetHealthCheckLogsClearingLogs,
  selectSetHealthCheckLogsClearDialogOpen,
  selectSetHealthCheckLogsDetailDialogOpen,
  selectSetHealthCheckLogsFilter,
  selectSetHealthCheckLogsLoading,
  selectSetHealthCheckLogsLogs,
  selectSetHealthCheckLogsModels,
  selectSetHealthCheckLogsPage,
  selectSetHealthCheckLogsPages,
  selectSetHealthCheckLogsPageSize,
  selectSetHealthCheckLogsProviders,
  selectSetHealthCheckLogsResultDialogOpen,
  selectSetHealthCheckLogsTotal,
  selectSetHealthCheckLogsCurrentBatchId,
  useHealthCheckLogsPageStore,
} from "@/stores/health-check-logs";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

export function useHealthCheckLogsPage() {
  const loading = useHealthCheckLogsPageStore(selectHealthCheckLogsLoading);
  const logs = useHealthCheckLogsPageStore(selectHealthCheckLogsLogs);
  const providers = useHealthCheckLogsPageStore(selectHealthCheckLogsProviders);
  const models = useHealthCheckLogsPageStore(selectHealthCheckLogsModels);
  const filters = useHealthCheckLogsPageStore(selectHealthCheckLogsFilters);
  const page = useHealthCheckLogsPageStore(selectHealthCheckLogsPage);
  const pageSize = useHealthCheckLogsPageStore(selectHealthCheckLogsPageSize);
  const total = useHealthCheckLogsPageStore(selectHealthCheckLogsTotal);
  const pages = useHealthCheckLogsPageStore(selectHealthCheckLogsPages);
  const detailLog = useHealthCheckLogsPageStore(selectHealthCheckLogsDetailLog);
  const detailDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsDetailDialogOpen);
  const clearDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsClearDialogOpen);
  const clearingLogs = useHealthCheckLogsPageStore(selectHealthCheckLogsClearingLogs);
  const resultDialogOpen = useHealthCheckLogsPageStore(selectHealthCheckLogsResultDialogOpen);
  const currentBatchId = useHealthCheckLogsPageStore(selectHealthCheckLogsCurrentBatchId);
  const backgroundBatchId = useHealthCheckLogsPageStore(selectHealthCheckLogsBackgroundBatchId);
  const backgroundCheckComplete = useHealthCheckLogsPageStore(selectHealthCheckLogsBackgroundCheckComplete);

  const setLoading = useHealthCheckLogsPageStore(selectSetHealthCheckLogsLoading);
  const setLogs = useHealthCheckLogsPageStore(selectSetHealthCheckLogsLogs);
  const setProviders = useHealthCheckLogsPageStore(selectSetHealthCheckLogsProviders);
  const setModels = useHealthCheckLogsPageStore(selectSetHealthCheckLogsModels);
  const setFilter = useHealthCheckLogsPageStore(selectSetHealthCheckLogsFilter);
  const setPage = useHealthCheckLogsPageStore(selectSetHealthCheckLogsPage);
  const setPageSize = useHealthCheckLogsPageStore(selectSetHealthCheckLogsPageSize);
  const setTotal = useHealthCheckLogsPageStore(selectSetHealthCheckLogsTotal);
  const setPages = useHealthCheckLogsPageStore(selectSetHealthCheckLogsPages);
  const openDetailDialog = useHealthCheckLogsPageStore(selectOpenHealthCheckLogsDetailDialog);
  const setDetailDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsDetailDialogOpen);
  const setClearDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsClearDialogOpen);
  const setClearingLogs = useHealthCheckLogsPageStore(selectSetHealthCheckLogsClearingLogs);
  const setResultDialogOpen = useHealthCheckLogsPageStore(selectSetHealthCheckLogsResultDialogOpen);
  const setCurrentBatchId = useHealthCheckLogsPageStore(selectSetHealthCheckLogsCurrentBatchId);
  const setBackgroundBatchId = useHealthCheckLogsPageStore(selectSetHealthCheckLogsBackgroundBatchId);
  const setBackgroundCheckComplete = useHealthCheckLogsPageStore(selectSetHealthCheckLogsBackgroundCheckComplete);
  const resetTransient = useHealthCheckLogsPageStore(selectResetHealthCheckLogsTransient);

  const pollingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fetchLogsRef = useRef<(pageToFetch?: number, pageSizeToUse?: number) => Promise<void>>(
    async () => {}
  );
  const resultDialogOpenRef = useRef(resultDialogOpen);

  const stopBackgroundPolling = useCallback(() => {
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }
  }, []);

  const fetchFilterOptions = useCallback(async () => {
    const [providerResult, modelResult] = await Promise.allSettled([getProviders(), getModels()]);

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
  }, [setModels, setProviders]);

  const fetchLogs = useCallback(
    async (pageToFetch = page, pageSizeToUse: number = pageSize) => {
      setLoading(true);
      try {
        const result = await getHealthCheckLogs(
          pageToFetch,
          pageSizeToUse,
          toHealthCheckLogsApiFilters(filters)
        );
        setLogs(result.data);
        setTotal(result.total);
        setPages(result.pages);
      } catch (error) {
        console.error("Error fetching health check logs:", error);
      } finally {
        setLoading(false);
      }
    },
    [filters, page, pageSize, setLoading, setLogs, setPages, setTotal]
  );

  useEffect(() => {
    void fetchFilterOptions();
  }, [fetchFilterOptions]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

  useEffect(
    () => () => {
      stopBackgroundPolling();
      resetTransient();
    },
    [resetTransient, stopBackgroundPolling]
  );

  useEffect(() => {
    fetchLogsRef.current = fetchLogs;
  }, [fetchLogs]);

  useEffect(() => {
    resultDialogOpenRef.current = resultDialogOpen;
  }, [resultDialogOpen]);

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

  const refreshLogs = () => {
    void fetchLogs();
  };

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
  };

  const confirmClearLogs = async () => {
    try {
      setClearingLogs(true);
      const result = await clearHealthCheckLogs();
      toast.success(`已清空 ${result.deleted} 条健康检测日志`);
      setClearDialogOpen(false);
      setPage(1);
      await fetchLogs(1, pageSize);
    } catch (error) {
      toast.error(`清空健康检测日志失败: ${toErrorMessage(error)}`);
    } finally {
      setClearingLogs(false);
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
              void fetchLogsRef.current();
            }
          }
        } catch (error) {
          console.error("Failed to fetch background batch status:", error);
        }
      }, 3000);
    },
    [setBackgroundCheckComplete, stopBackgroundPolling]
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
      void fetchLogs();
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
    clearingLogs,
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
