import { apiRequest } from "../../core/client";

export interface PromptTokensDetails {
  cached_tokens: number;
  audio_tokens: number;
}

export interface CompletionTokensDetails {
  reasoning_tokens: number;
  audio_tokens: number;
}

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
  completion_tokens_details: CompletionTokensDetails;
  RequestHeaders?: string;
  RequestBody?: string;
  RawRequestBody?: string;
  ResponseHeaders?: string;
  ResponseBody?: string;
  RawResponseBody?: string;
  is_virtual_model: boolean;
  has_format_conversion: boolean;
  source_format?: string;
  target_format?: string;
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
  return apiRequest<string[]>("/user-agents");
}

export async function getLogs(
  page = 1,
  pageSize = 20,
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

  if (filters.name) {
    params.append("name", filters.name);
  }
  if (filters.providerModel) {
    params.append("provider_model", filters.providerModel);
  }
  if (filters.providerName) {
    params.append("provider_name", filters.providerName);
  }
  if (filters.status) {
    params.append("status", filters.status);
  }
  if (filters.style) {
    params.append("style", filters.style);
  }
  if (filters.userAgent) {
    params.append("user_agent", filters.userAgent);
  }

  return apiRequest<LogsResponse>(`/logs?${params.toString()}`);
}

export async function getLogDetail(logId: number): Promise<ChatLog> {
  return apiRequest<ChatLog>(`/logs/${logId}`);
}

export interface DiffEntry {
  path: string;
  raw: unknown;
  after: unknown;
}

export interface DiffResult {
  lost_fields: DiffEntry[];
  added_fields: DiffEntry[];
  changed_values: DiffEntry[];
}

export async function getLogDiff(logId: number): Promise<DiffResult> {
  return apiRequest<DiffResult>(`/logs/${logId}/diff`);
}

export async function getChatIO(logId: number): Promise<ChatIO> {
  return apiRequest<ChatIO>(`/logs/${logId}/chat-io`);
}

export async function deleteLog(id: number): Promise<void> {
  await apiRequest<void>(`/logs/${id}`, {
    method: "DELETE",
  });
}

export async function batchDeleteLogs(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/logs/batch", {
    method: "DELETE",
    body: JSON.stringify({ ids }),
  });
}

export async function clearAllLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/logs/clear", {
    method: "DELETE",
  });
}

export async function clearFilteredLogs(filters: {
  name?: string;
  providerName?: string;
  status?: string;
  style?: string;
  userAgent?: string;
}): Promise<{ deleted: number }> {
  const params = new URLSearchParams();
  if (filters.name) {
    params.append("name", filters.name);
  }
  if (filters.providerName) {
    params.append("provider_name", filters.providerName);
  }
  if (filters.status) {
    params.append("status", filters.status);
  }
  if (filters.style) {
    params.append("style", filters.style);
  }
  if (filters.userAgent) {
    params.append("user_agent", filters.userAgent);
  }

  const queryString = params.toString();
  const endpoint = queryString ? `/logs/clear-filtered?${queryString}` : "/logs/clear-filtered";
  return apiRequest<{ deleted: number }>(endpoint, {
    method: "DELETE",
  });
}
