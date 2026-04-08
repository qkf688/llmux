import { apiRequest } from "../../core/client";

export interface SystemConfig {
  enable_smart_routing: boolean;
  success_rate_weight: number;
  response_time_weight: number;
  decay_threshold_hours: number;
  min_weight: number;
}

export interface SystemStatus {
  total_providers: number;
  total_models: number;
  active_requests: number;
  uptime: string;
  version: string;
}

export interface ProviderMetric {
  provider_id: number;
  provider_name: string;
  success_rate: number;
  avg_response_time: number;
  total_requests: number;
  total_tokens: number;
  success_count: number;
  failure_count: number;
}

export async function getSystemStatus(): Promise<SystemStatus> {
  return apiRequest<SystemStatus>("/status");
}

export async function getProviderMetrics(): Promise<ProviderMetric[]> {
  return apiRequest<ProviderMetric[]>("/metrics/providers");
}

export async function getSystemConfig(): Promise<SystemConfig> {
  return apiRequest<SystemConfig>("/config");
}

export async function updateSystemConfig(config: SystemConfig): Promise<SystemConfig> {
  return apiRequest<SystemConfig>("/config", {
    method: "PUT",
    body: JSON.stringify(config),
  });
}
