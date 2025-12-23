// API client for interacting with the backend

const API_BASE = '/api';

export interface Provider {
  ID: number;
  Name: string;
  Type: string;
  Config: string;
  Console: string;
  Proxy: string;
  ModelEndpoint?: boolean;
}

export interface Model {
  ID: number;
  Name: string;
  Remark: string;
  MaxRetry: number;
  TimeOut: number;
  IOLog: boolean;
}

export interface ModelWithProvider {
  ID: number;
  ModelID: number;
  ProviderModel: string;
  ProviderID: number;
  ToolCall: boolean;
  StructuredOutput: boolean;
  Image: boolean;
  WithHeader: boolean;
  CustomerHeaders: Record<string, string> | null;
  Status: boolean | null;
  Weight: number;
  Priority: number;
}

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
  success_count: number;
  failure_count: number;
}

// Generic API request function
async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`;

  // Get token from localStorage
  const token = localStorage.getItem("authToken");

  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      ...options.headers,
    },
    ...options,
  });

  // Handle 401 Unauthorized response
  if (response.status === 401) {
    // Redirect to login page
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!response.ok) {
    throw new Error(`API request failed: ${response.status} ${response.statusText}`);
  }

  const data = await response.json();
  if (data.code !== 200) {
    throw new Error(`${data.message}`);
  }
  return data.data as T;
}

// Provider API functions
export async function getProviders(filters: {
  name?: string;
  type?: string;
} = {}): Promise<Provider[]> {
  const params = new URLSearchParams();

  if (filters.name) params.append("name", filters.name);
  if (filters.type) params.append("type", filters.type);

  const queryString = params.toString();
  const endpoint = queryString ? `/providers?${queryString}` : '/providers';

  return apiRequest<Provider[]>(endpoint);
}

export async function createProvider(provider: {
  name: string;
  type: string;
  config: string;
  console: string;
  proxy: string;
  model_endpoint?: boolean;
}): Promise<Provider> {
  return apiRequest<Provider>('/providers', {
    method: 'POST',
    body: JSON.stringify(provider),
  });
}

export async function updateProvider(id: number, provider: {
  name?: string;
  type?: string;
  config?: string;
  console?: string;
  proxy?: string;
  model_endpoint?: boolean;
}): Promise<Provider> {
  return apiRequest<Provider>(`/providers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(provider),
  });
}

export async function deleteProvider(id: number): Promise<void> {
  await apiRequest<void>(`/providers/${id}`, {
    method: 'DELETE',
  });
}

// Model API functions
export async function getModels(): Promise<Model[]> {
  return apiRequest<Model[]>('/models');
}

export async function createModel(model: {
  name: string;
  remark: string;
  max_retry: number;
  time_out: number;
  io_log: boolean;
}): Promise<Model> {
  return apiRequest<Model>('/models', {
    method: 'POST',
    body: JSON.stringify(model),
  });
}

export async function updateModel(id: number, model: {
  name?: string;
  remark?: string;
  max_retry?: number;
  time_out?: number;
  io_log?: boolean;
}): Promise<Model> {
  return apiRequest<Model>(`/models/${id}`, {
    method: 'PUT',
    body: JSON.stringify(model),
  });
}

export async function deleteModel(id: number): Promise<void> {
  await apiRequest<void>(`/models/${id}`, {
    method: 'DELETE',
  });
}

export async function batchDeleteModels(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/models/batch', {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  });
}

// Model-Provider API functions
export async function getModelProviders(modelId: number): Promise<ModelWithProvider[]> {
  return apiRequest<ModelWithProvider[]>(`/model-providers?model_id=${modelId}`);
}

export async function getModelProviderHealthStatus(modelProviderId: number, limit: number = 10): Promise<boolean[]> {
  const params = new URLSearchParams({
    model_provider_id: modelProviderId.toString(),
    limit: limit.toString()
  });
  return apiRequest<boolean[]>(`/model-providers/health-status?${params.toString()}`);
}

export async function getModelProviderStatus(providerId: number, modelName: string, providerModel: string): Promise<boolean[]> {
  const params = new URLSearchParams({
    provider_id: providerId.toString(),
    model_name: modelName,
    provider_model: providerModel
  });
  return apiRequest<boolean[]>(`/model-providers/status?${params.toString()}`);
}

export async function createModelProvider(association: {
  model_id: number;
  provider_name: string;
  provider_id: number;
  tool_call: boolean;
  structured_output: boolean;
  image: boolean;
  with_header: boolean;
  customer_headers: Record<string, string>;
  weight: number;
  priority?: number;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>('/model-providers', {
    method: 'POST',
    body: JSON.stringify(association),
  });
}

export async function updateModelProvider(id: number, association: {
  model_id?: number;
  provider_name?: string;
  provider_id?: number;
  tool_call?: boolean;
  structured_output?: boolean;
  image?: boolean;
  with_header?: boolean;
  customer_headers?: Record<string, string>;
  weight?: number;
  priority?: number;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(association),
  });
}

export async function updateModelProviderStatus(id: number, status: boolean): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

export async function deleteModelProvider(id: number): Promise<void> {
  await apiRequest<void>(`/model-providers/${id}`, {
    method: 'DELETE',
  });
}

export async function batchDeleteModelProviders(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/model-providers/batch', {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  });
}

// System API functions
export async function getSystemStatus(): Promise<SystemStatus> {
  return apiRequest<SystemStatus>('/status');
}

export async function getProviderMetrics(): Promise<ProviderMetric[]> {
  return apiRequest<ProviderMetric[]>('/metrics/providers');
}

export async function getSystemConfig(): Promise<SystemConfig> {
  return apiRequest<SystemConfig>('/config');
}

export async function updateSystemConfig(config: SystemConfig): Promise<SystemConfig> {
  return apiRequest<SystemConfig>('/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
}

// Metrics API functions
export interface MetricsData {
  reqs: number;
  tokens: number;
}

export interface ModelCount {
  model: string;
  calls: number;
}

export async function getMetrics(days: number): Promise<MetricsData> {
  return apiRequest<MetricsData>(`/metrics/use/${days}`);
}

export async function getModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>('/metrics/counts');
}

// Test API functions
export async function testModelProvider(id: number): Promise<any> {
  return apiRequest<any>(`/test/${id}`);
}

export async function testProviderModel(providerId: number, model: string): Promise<any> {
  return apiRequest<any>(`/providers/${providerId}/test`, {
    method: "POST",
    body: JSON.stringify({ model }),
  });
}

// Provider Templates API functions
export interface ProviderTemplate {
  type: string;
  template: string;
}

export async function getProviderTemplates(): Promise<ProviderTemplate[]> {
  return apiRequest<ProviderTemplate[]>('/providers/template');
}

// Provider Models API functions
export interface ProviderModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

export async function getProviderModels(providerId: number, options: { source?: "upstream" | "all" } = {}): Promise<ProviderModel[]> {
  const params = new URLSearchParams();
  if (options.source) {
    params.append("source", options.source);
  }
  const queryString = params.toString();
  const endpoint = queryString ? `/providers/models/${providerId}?${queryString}` : `/providers/models/${providerId}`;
  return apiRequest<ProviderModel[]>(endpoint);
}

// Logs API functions
export interface ChatLog {
  ID: number;
  CreatedAt: string;
  Name: string;
  ProviderModel: string;
  ProviderName: string;
  Status: string;
  Style: string;
  UserAgent: string;
  RemoteIP?: string;
  Error: string;
  Retry: number;
  ProxyTime: number;
  FirstChunkTime: number;
  ChunkTime: number;
  Tps: number;
  ChatIO: boolean;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  prompt_tokens_details: PromptTokensDetails;
  // 新增字段：原始请求和响应内容
  RequestHeaders?: string;
  RequestBody?: string;
  ResponseHeaders?: string;
  ResponseBody?: string;
  RawResponseBody?: string; // 原始响应体（转换前）
}

export interface PromptTokensDetails {
  cached_tokens: number;
}

export interface ChatIO {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: unknown;
  LogId: number;
  Input: string;
  OfString?: string | null;
  OfStringArray?: string[] | null;
}

export interface LogsResponse {
  data: ChatLog[];
  total: number;
  page: number;
  page_size: number;
  pages: number;
}

export async function getUserAgents(): Promise<string[]> {
  return apiRequest<string[]>('/user-agents');
}

export async function getLogs(
  page: number = 1,
  pageSize: number = 20,
  filters: {
    name?: string;
    providerModel?: string;
    providerName?: string;
    status?: string;
    style?: string;
    userAgent?: string;
  } = {}
): Promise<LogsResponse> {
  const params = new URLSearchParams();
  params.append("page", page.toString());
  params.append("page_size", pageSize.toString());

  if (filters.name) params.append("name", filters.name);
  if (filters.providerModel) params.append("provider_model", filters.providerModel);
  if (filters.providerName) params.append("provider_name", filters.providerName);
  if (filters.status) params.append("status", filters.status);
  if (filters.style) params.append("style", filters.style);
  if (filters.userAgent) params.append("user_agent", filters.userAgent);

  return apiRequest<LogsResponse>(`/logs?${params.toString()}`);
}

export async function getChatIO(logId: number): Promise<ChatIO> {
  return apiRequest<ChatIO>(`/logs/${logId}/chat-io`);
}

// Settings API functions
export interface RawLogOptions {
  request_headers: boolean;
  request_body: boolean;
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
  disable_all_logs: boolean;
  count_health_check_as_success: boolean;
  count_health_check_as_failure: boolean;
  // 性能优化相关设置
  disable_performance_tracking: boolean;
  disable_token_counting: boolean;
  enable_request_trace: boolean;
  strip_response_headers: boolean;
  enable_format_conversion: boolean;
  // 模型同步相关设置
  model_sync_enabled: boolean;
  model_sync_interval: number;
  model_sync_log_retention_count: number;
  model_sync_log_retention_days: number;
  // 模型关联相关设置
  auto_associate_on_add: boolean;
  auto_clean_on_delete: boolean;
}

export async function getSettings(): Promise<Settings> {
  return apiRequest<Settings>('/settings');
}

export async function updateSettings(settings: Settings): Promise<Settings> {
  return apiRequest<Settings>('/settings', {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
}

export interface ResetWeightsResponse {
  updated: number;
  default_weight: number;
}

export async function resetModelWeights(modelId?: number): Promise<ResetWeightsResponse> {
  return apiRequest<ResetWeightsResponse>('/settings/reset-weights', {
    method: 'POST',
    body: JSON.stringify({ model_id: modelId }),
  });
}

export interface ResetPrioritiesResponse {
  updated: number;
  default_priority: number;
}

export async function resetModelPriorities(modelId?: number): Promise<ResetPrioritiesResponse> {
  return apiRequest<ResetPrioritiesResponse>('/settings/reset-priorities', {
    method: 'POST',
    body: JSON.stringify({ model_id: modelId }),
  });
}

export interface EnableAssociationsResponse {
  updated: number;
}

export async function enableAllAssociations(modelId?: number): Promise<EnableAssociationsResponse> {
  return apiRequest<EnableAssociationsResponse>('/settings/enable-all-associations', {
    method: 'POST',
    body: JSON.stringify({ model_id: modelId }),
  });
}

export interface AssociationPreview {
  model_id: number;
  model_name: string;
  provider_id: number;
  provider_name: string;
  provider_model: string;
}

export interface ModelTemplateItem {
  name: string;
  sources: string[];
}

export interface ModelTemplate {
  model_id: number;
  model_name: string;
  items: ModelTemplateItem[];
}

export async function getModelTemplate(modelId: number): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template`);
}

export async function addModelTemplateItem(modelId: number, name: string): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template/items`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
}

export async function deleteModelTemplateItem(modelId: number, name: string): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template/items`, {
    method: 'DELETE',
    body: JSON.stringify({ name }),
  });
}

export async function previewAutoAssociate(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>('/model-providers/auto-associate/preview');
}

export async function autoAssociateModels(): Promise<{ added: number }> {
  return apiRequest<{ added: number }>('/model-providers/auto-associate', {
    method: 'POST',
  });
}

export async function previewCleanInvalid(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>('/model-providers/clean-invalid/preview');
}

export async function cleanInvalidAssociations(): Promise<{ removed: number }> {
  return apiRequest<{ removed: number }>('/model-providers/clean-invalid', {
    method: 'POST',
  });
}

// Log management API functions
export async function deleteLog(id: number): Promise<void> {
  await apiRequest<void>(`/logs/${id}`, {
    method: 'DELETE',
  });
}

export async function batchDeleteLogs(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/logs/batch', {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  });
}

export async function clearAllLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/logs/clear', {
    method: 'DELETE',
  });
}

// Maintenance API functions
export async function vacuumDatabase(): Promise<{ message: string }> {
  return apiRequest<{ message: string }>('/maintenance/vacuum', {
    method: 'POST',
  });
}

// Health Check API functions
export interface HealthCheckSettings {
  enabled: boolean;
  interval: number;
  failure_threshold: number;
  failure_disable_enabled: boolean;
  auto_enable: boolean;
  log_retention_count: number;
  count_health_check_as_success: boolean;
  count_health_check_as_failure: boolean;
}

export interface HealthCheckLog {
  ID: number;
  CreatedAt: string;
  batch_id?: string;
  model_provider_id: number;
  model_name: string;
  provider_name: string;
  provider_model: string;
  status: string;
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
  pending: number;
  completed: boolean;
  logs: HealthCheckLog[];
}

export async function getHealthCheckSettings(): Promise<HealthCheckSettings> {
  return apiRequest<HealthCheckSettings>('/health-check/settings');
}

export async function updateHealthCheckSettings(settings: HealthCheckSettings): Promise<HealthCheckSettings> {
  return apiRequest<HealthCheckSettings>('/health-check/settings', {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
}

export async function getHealthCheckLogs(
  page: number = 1,
  pageSize: number = 20,
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

  if (filters.modelProviderId) params.append("model_provider_id", filters.modelProviderId.toString());
  if (filters.modelName) params.append("model_name", filters.modelName);
  if (filters.providerName) params.append("provider_name", filters.providerName);
  if (filters.status) params.append("status", filters.status);

  return apiRequest<HealthCheckLogsResponse>(`/health-check/logs?${params.toString()}`);
}

export async function clearHealthCheckLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/health-check/logs', {
    method: 'DELETE',
  });
}

export async function runHealthCheck(id: number): Promise<HealthCheckLog> {
  return apiRequest<HealthCheckLog>(`/health-check/run/${id}`, {
    method: 'POST',
  });
}

export async function runHealthCheckAll(): Promise<{ batch_id: string; message: string }> {
  return apiRequest<{ batch_id: string; message: string }>('/health-check/run-all', {
    method: 'POST',
  });
}

export async function getBatchHealthCheckStatus(batchId: string): Promise<BatchHealthCheckStatus> {
  return apiRequest<BatchHealthCheckStatus>(`/health-check/batch/${batchId}`);
}

// Model Sync API functions

export interface ModelSyncLog {
  ID: number;
  ProviderID: number;
  ProviderName: string;
  AddedCount: number;
  RemovedCount: number;
  AddedModels: string[];
  RemovedModels: string[];
  SyncedAt: string;
}

export interface ModelSyncLogsResponse {
  data: ModelSyncLog[];
  pagination: {
    page: number;
    page_size: number;
    total: number;
    total_pages: number;
  };
}

export async function syncProviderModels(providerId: number): Promise<ModelSyncLog | { message: string }> {
  return apiRequest<ModelSyncLog | { message: string }>(`/model-sync/${providerId}`, {
    method: 'POST',
  });
}

export async function syncAllProviderModels(): Promise<{
  message?: string;
  logs?: ModelSyncLog[];
  synced_providers?: number;
  added_total?: number;
  removed_total?: number;
}> {
  return apiRequest(`/model-sync/all`, {
    method: 'POST',
  });
}

export async function getModelSyncLogs(params: {
  page?: number;
  page_size?: number;
  provider_id?: number;
} = {}): Promise<ModelSyncLogsResponse> {
  const queryParams = new URLSearchParams();
  if (params.page) queryParams.append('page', params.page.toString());
  if (params.page_size) queryParams.append('page_size', params.page_size.toString());
  if (params.provider_id) queryParams.append('provider_id', params.provider_id.toString());

  const queryString = queryParams.toString();
  const endpoint = queryString ? `/model-sync/logs?${queryString}` : '/model-sync/logs';

  return apiRequest<ModelSyncLogsResponse>(endpoint);
}

export async function deleteModelSyncLogs(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/model-sync/logs', {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  });
}

export async function clearModelSyncLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>('/model-sync/logs/clear', {
    method: 'DELETE',
  });
}

// Database Stats API functions
export interface TableStat {
  name: string;
  display_name: string;
  count: number;
  estimated_size_human: string;
}

export interface DatabaseStats {
  file_path: string;
  file_size: number;
  file_size_human: string;
  table_stats: TableStat[];
  page_count: number;
  page_size: number;
  free_pages: number;
  last_vacuum_at?: string;
  can_vacuum: boolean;
  // Additional fields for database info section
  db_path: string;
  sqlite_version: string;
  encoding: string;
  last_modified: string;
}

export async function getDatabaseStats(): Promise<DatabaseStats> {
  return apiRequest<DatabaseStats>('/system/database-stats');
}
