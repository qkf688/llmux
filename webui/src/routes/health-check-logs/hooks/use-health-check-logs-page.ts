import { useCallback, useEffect, useRef, useState } from "react";
import {
  clearHealthCheckLogs,
  getBatchHealthCheckStatus,
  getHealthCheckLogs,
  getModels,
  getProviders,
  runHealthCheckAll,
  type HealthCheckLog,
  type Model,
  type Provider,
} from "@/lib/api";
import { toast } from "sonner";
import { DEFAULT_HEALTH_CHECK_LOGS_FILTERS, type HealthCheckLogsFilters } from "../types";
import { toHealthCheckLogsApiFilters } from "../utils/filters";
import {
  clearStoredHealthCheckBatchState,
  markStoredHealthCheckBatchCompleted,
  readStoredHealthCheckBatchState,
  writeStoredHealthCheckBatchState,
} from "../utils/storage";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

export function useHealthCheckLogsPage() {
  const [loading, setLoading] = useState(true);
  const [logs, setLogs] = useState<HealthCheckLog[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);

  const [filters, setFilters] = useState<HealthCheckLogsFilters>({
    ...DEFAULT_HEALTH_CHECK_LOGS_FILTERS,
  });
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  const [detailLog, setDetailLog] = useState<HealthCheckLog | null>(null);
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [clearDialogOpen, setClearDialogOpen] = useState(false);
  const [clearingLogs, setClearingLogs] = useState(false);

  const [resultDialogOpen, setResultDialogOpen] = useState(false);
  const [currentBatchId, setCurrentBatchId] = useState<string | null>(null);
  const [backgroundBatchId, setBackgroundBatchId] = useState<string | null>(null);
  const [backgroundCheckComplete, setBackgroundCheckComplete] = useState(false);

  const pollingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fetchLogsRef = useRef<(pageToFetch?: number, pageSizeToUse?: number) => Promise<void>>(
    async () => {}
  );
  const resultDialogOpenRef = useRef(resultDialogOpen);

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
  }, []);

  const fetchLogs = useCallback(
    async (pageToFetch = page, pageSizeToUse = pageSize) => {
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
    [filters, page, pageSize]
  );

  useEffect(() => {
    void fetchFilterOptions();
  }, [fetchFilterOptions]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

  useEffect(() => {
    fetchLogsRef.current = fetchLogs;
  }, [fetchLogs]);

  useEffect(() => {
    resultDialogOpenRef.current = resultDialogOpen;
  }, [resultDialogOpen]);

  const handleFilterChange = (key: keyof HealthCheckLogsFilters, value: string) => {
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

  const openDetailDialog = (log: HealthCheckLog) => {
    setDetailLog(log);
    setDetailDialogOpen(true);
  };

  const handleDetailDialogOpenChange = (open: boolean) => {
    setDetailDialogOpen(open);
    if (!open) {
      setDetailLog(null);
    }
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

  const stopBackgroundPolling = useCallback(() => {
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }
  }, []);

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
    [stopBackgroundPolling]
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
  }, [startBackgroundPolling]);

  useEffect(
    () => () => {
      stopBackgroundPolling();
    },
    [stopBackgroundPolling]
  );

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
