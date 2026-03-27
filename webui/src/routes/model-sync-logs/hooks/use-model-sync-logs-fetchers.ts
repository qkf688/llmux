import { useCallback, useEffect } from "react";
import {
  getModelSyncLogs,
  getModelSyncStats,
  getProviders,
  getRecentAddedModels,
} from "@/lib/api";
import { toast } from "sonner";
import type { ModelSyncTab } from "../types";
import type { ModelSyncLogsPageState } from "@/stores/model-sync-logs";

const LOG_PAGE_SIZE = 20;

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

type UseModelSyncLogsFetchersInput = {
  activeTab: ModelSyncTab;
  page: ModelSyncLogsPageState["page"];
  showUnchanged: ModelSyncLogsPageState["showUnchanged"];
  setStats: ModelSyncLogsPageState["setStats"];
  setStatsLoading: ModelSyncLogsPageState["setStatsLoading"];
  setLogs: ModelSyncLogsPageState["setLogs"];
  setTotalPages: ModelSyncLogsPageState["setTotalPages"];
  setLoading: ModelSyncLogsPageState["setLoading"];
  setRecentModels: ModelSyncLogsPageState["setRecentModels"];
  setSyncTime: ModelSyncLogsPageState["setSyncTime"];
  setRecentLoading: ModelSyncLogsPageState["setRecentLoading"];
  setRecentErrors: ModelSyncLogsPageState["setRecentErrors"];
  setProvidersById: ModelSyncLogsPageState["setProvidersById"];
  setErrorsLoading: ModelSyncLogsPageState["setErrorsLoading"];
};

export function useModelSyncLogsFetchers({
  activeTab,
  page,
  showUnchanged,
  setStats,
  setStatsLoading,
  setLogs,
  setTotalPages,
  setLoading,
  setRecentModels,
  setSyncTime,
  setRecentLoading,
  setRecentErrors,
  setProvidersById,
  setErrorsLoading,
}: UseModelSyncLogsFetchersInput) {
  const fetchStats = useCallback(async () => {
    try {
      setStatsLoading(true);
      const data = await getModelSyncStats();
      setStats(data);
    } catch (error) {
      console.error("加载统计信息失败:", error);
    } finally {
      setStatsLoading(false);
    }
  }, [setStats, setStatsLoading]);

  const fetchLogs = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getModelSyncLogs({
        page,
        page_size: LOG_PAGE_SIZE,
        show_unchanged: showUnchanged,
      });
      setLogs(data.data ?? []);
      setTotalPages(data.pagination?.total_pages ?? 1);
    } catch (error) {
      toast.error(`加载日志失败: ${toErrorMessage(error)}`);
    } finally {
      setLoading(false);
    }
  }, [page, setLoading, setLogs, setTotalPages, showUnchanged]);

  const fetchRecentModels = useCallback(async () => {
    try {
      setRecentLoading(true);
      const data = await getRecentAddedModels();
      setRecentModels(data.data ?? []);
      setSyncTime(data.sync_time || "");
    } catch (error) {
      toast.error(`加载最近新增模型失败: ${toErrorMessage(error)}`);
    } finally {
      setRecentLoading(false);
    }
  }, [setRecentLoading, setRecentModels, setSyncTime]);

  const fetchRecentErrors = useCallback(async () => {
    try {
      setErrorsLoading(true);
      const [logsData, providers] = await Promise.all([
        getModelSyncLogs({ page: 1, page_size: 100, status: "error" }),
        getProviders(),
      ]);

      setRecentErrors(logsData.data ?? []);
      setProvidersById(Object.fromEntries((providers ?? []).map((provider) => [provider.ID, provider])));
    } catch (error) {
      toast.error(`加载最近错误失败: ${toErrorMessage(error)}`);
    } finally {
      setErrorsLoading(false);
    }
  }, [setErrorsLoading, setProvidersById, setRecentErrors]);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    if (activeTab === "logs") {
      void fetchLogs();
    }
  }, [activeTab, fetchLogs]);

  useEffect(() => {
    if (activeTab === "recent") {
      void fetchRecentModels();
    }
  }, [activeTab, fetchRecentModels]);

  useEffect(() => {
    if (activeTab === "errors") {
      void fetchRecentErrors();
    }
  }, [activeTab, fetchRecentErrors]);

  const refreshCurrentTab = useCallback(() => {
    if (activeTab === "logs") {
      void fetchLogs();
      return;
    }
    if (activeTab === "recent") {
      void fetchRecentModels();
      return;
    }
    void fetchRecentErrors();
  }, [activeTab, fetchLogs, fetchRecentErrors, fetchRecentModels]);

  return {
    fetchStats,
    fetchLogs,
    fetchRecentModels,
    fetchRecentErrors,
    refreshCurrentTab,
  };
}

