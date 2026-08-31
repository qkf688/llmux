import { apiRequest } from "../../core/client";

export interface RawLogOptions {
  request_headers: boolean;
  request_body: boolean;
  raw_request_body: boolean;
  response_headers: boolean;
  response_body: boolean;
  raw_response_body: boolean;
}

export interface Settings {
  strict_capability_match: boolean;
  auto_weight_decay: boolean;
  auto_weight_decay_default: number;
  auto_weight_decay_step: number;
  auto_success_increase: boolean;
  auto_weight_increase_step: number;
  auto_weight_increase_max: number;
  auto_priority_decay: boolean;
  auto_priority_decay_default: number;
  auto_priority_decay_step: number;
  auto_priority_decay_threshold: number;
  auto_priority_decay_disable_enabled: boolean;
  auto_priority_increase_step: number;
  auto_priority_increase_max: number;
  consecutive_failure_threshold: number;
  consecutive_failure_disable_enabled: boolean;
  log_retention_count: number;
  log_raw_request_response: RawLogOptions;
  log_raw_request_response_errors_only: boolean;
  disable_all_logs: boolean;
  count_health_check_as_success: boolean;
  count_health_check_as_failure: boolean;
  disable_performance_tracking: boolean;
  disable_token_counting: boolean;
  enable_request_trace: boolean;
  strip_response_headers: boolean;
  enable_format_conversion: boolean;
  model_sync_enabled: boolean;
  model_sync_interval: number;
  model_sync_log_retention_count: number;
  model_sync_log_retention_days: number;
  model_sync_filter_rules: string[];
  template_fuzzy_match_enabled: boolean;
  template_fuzzy_match_separators: string[];
  template_fuzzy_match_suffixes: string[];
  auto_associate_on_add: boolean;
  auto_clean_on_delete: boolean;
  auto_save_template_on_associate: boolean;
  reasoning_effort_mapping_enabled: boolean;
  reasoning_effort_default_value: "minimal" | "low" | "medium" | "high" | "xhigh" | "max";
  reasoning_effort_unknown_strategy: "clamp_to_default" | "passthrough";
  request_header_timeout: number;
  request_total_timeout: number;
  stream_first_byte_timeout: number;
  request_max_retry: number;
  cred_health_cooldown_429_sec: number;
  cred_health_cooldown_server_sec: number;
  cred_health_auth_fail_threshold: number;
  cred_health_probe_interval_sec: number;
}

export interface ResetWeightsResponse {
  updated: number;
  default_weight: number;
}

export interface ResetPrioritiesResponse {
  updated: number;
  default_priority: number;
}

export interface EnableAssociationsResponse {
  updated: number;
}

export async function getSettings(): Promise<Settings> {
  return apiRequest<Settings>("/settings");
}

export async function updateSettings(settings: Settings): Promise<Settings> {
  return apiRequest<Settings>("/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export async function resetModelWeights(modelId?: number): Promise<ResetWeightsResponse> {
  return apiRequest<ResetWeightsResponse>("/settings/reset-weights", {
    method: "POST",
    body: JSON.stringify({ model_id: modelId }),
  });
}

export async function resetModelPriorities(modelId?: number): Promise<ResetPrioritiesResponse> {
  return apiRequest<ResetPrioritiesResponse>("/settings/reset-priorities", {
    method: "POST",
    body: JSON.stringify({ model_id: modelId }),
  });
}

export async function enableAllAssociations(modelId?: number): Promise<EnableAssociationsResponse> {
  return apiRequest<EnableAssociationsResponse>("/settings/enable-all-associations", {
    method: "POST",
    body: JSON.stringify({ model_id: modelId }),
  });
}
