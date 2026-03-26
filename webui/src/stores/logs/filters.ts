import type { LogsFilters } from "@/stores/logs/types";

export type ApiLogsFilters = {
  providerName?: string;
  name?: string;
  status?: string;
  style?: string;
  userAgent?: string;
};

export function toApiLogsFilters(filters: LogsFilters): ApiLogsFilters {
  return {
    providerName: filters.providerName === "all" ? undefined : filters.providerName,
    name: filters.model === "all" ? undefined : filters.model,
    status: filters.status === "all" ? undefined : filters.status,
    style: filters.style === "all" ? undefined : filters.style,
    userAgent: filters.userAgent === "all" ? undefined : filters.userAgent,
  };
}

export function hasActiveLogsFilters(filters: LogsFilters): boolean {
  return Object.values(filters).some((value) => value !== "all");
}

export function buildLogsFiltersSummary(filters: LogsFilters): string {
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
}

