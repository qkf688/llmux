import { useCallback, useEffect } from "react";
import {
  getLogs,
  getModels,
  getProviderTemplates,
  getProviders,
  getUserAgents,
} from "@/lib/api";
import { toast } from "sonner";
import type { LogsFilters } from "../types";
import { toApiLogsFilters, type LogsPageState } from "@/stores/logs";

const toErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

type UseLogsFetchersInput = {
  filters: LogsFilters;
  page: LogsPageState["page"];
  pageSize: LogsPageState["pageSize"];
  setLoading: LogsPageState["setLoading"];
  setLogs: LogsPageState["setLogs"];
  setTotal: LogsPageState["setTotal"];
  setPages: LogsPageState["setPages"];
  setProviders: LogsPageState["setProviders"];
  setModels: LogsPageState["setModels"];
  setUserAgents: LogsPageState["setUserAgents"];
  setAvailableStyles: LogsPageState["setAvailableStyles"];
};

export function useLogsFetchers({
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
}: UseLogsFetchersInput) {
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
      const styleTypes = Array.from(new Set(templateResult.value.map((template) => template.type).filter(Boolean)));
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

  return { fetchFilterOptions, fetchLogs };
}

