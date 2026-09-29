import { apiRequest } from "../../core/client";

export interface HealthCheckSettings {
  enabled: boolean;
  interval: number;
  failure_threshold: number;
  failure_disable_enabled: boolean;
  auto_enable: boolean;
  log_retention_count: number;
  count_health_check_as_success: boolean;
  count_health_check_as_failure: boolean;
  check_disabled_only: boolean;
}

// 健康检测状态三态（值域与后端 models.HealthCheckStatus* 常量同源）：
// success/error 是上游给出的结论，not_measured 是本机原因没测成
// （读不到库、没有启用的端点、没有可用凭据）——不是供应商的问题。
export type HealthCheckStatus = "success" | "error" | "not_measured";

export interface HealthCheckLog {
  ID: number;
  CreatedAt: string;
  batch_id?: string;
  model_provider_id: number;
  model_name: string;
  provider_name: string;
  provider_model: string;
  status: HealthCheckStatus;
  error: string;
  response_time: number;
  checked_at: string;
}

export interface HealthCheckLogsResponse {
  data: HealthCheckLog[];
  total: number;
  page: number;
  page_size: number;
  pages: number;
}

export interface BatchHealthCheckStatus {
  batch_id: string;
  total_count: number;
  success: number;
  failed: number;
  // 本机原因没测成的条数：既不算成功也不算失败，不单列则四项加不平 total_count
  not_measured: number;
  pending: number;
  completed: boolean;
  logs: HealthCheckLog[];
}

export async function getHealthCheckSettings(): Promise<HealthCheckSettings> {
  return apiRequest<HealthCheckSettings>("/health-check/settings");
}

export async function updateHealthCheckSettings(
  settings: HealthCheckSettings
): Promise<HealthCheckSettings> {
  return apiRequest<HealthCheckSettings>("/health-check/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export async function getHealthCheckLogs(
  page = 1,
  pageSize = 20,
  filters: {
    modelProviderId?: number;
    modelName?: string;
    providerName?: string;
    status?: string;
  } = {}
): Promise<HealthCheckLogsResponse> {
  const params = new URLSearchParams();
  params.append("page", page.toString());
  params.append("page_size", pageSize.toString());

  if (filters.modelProviderId) {
    params.append("model_provider_id", filters.modelProviderId.toString());
  }
  if (filters.modelName) {
    params.append("model_name", filters.modelName);
  }
  if (filters.providerName) {
    params.append("provider_name", filters.providerName);
  }
  if (filters.status) {
    params.append("status", filters.status);
  }

  return apiRequest<HealthCheckLogsResponse>(`/health-check/logs?${params.toString()}`);
}

export async function clearHealthCheckLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/health-check/logs", {
    method: "DELETE",
  });
}

export async function runHealthCheck(id: number): Promise<HealthCheckLog> {
  return apiRequest<HealthCheckLog>(`/health-check/run/${id}`, {
    method: "POST",
  });
}

export async function runHealthCheckAll(): Promise<{ batch_id: string; message: string }> {
  return apiRequest<{ batch_id: string; message: string }>("/health-check/run-all", {
    method: "POST",
  });
}

export async function getBatchHealthCheckStatus(batchId: string): Promise<BatchHealthCheckStatus> {
  return apiRequest<BatchHealthCheckStatus>(`/health-check/batch/${batchId}`);
}
