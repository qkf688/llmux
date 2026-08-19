import { apiRequest } from "../../core/client";

export interface PromptTokensDetails {
  cached_tokens: number;
  audio_tokens: number;
}

export interface CompletionTokensDetails {
  reasoning_tokens: number;
  audio_tokens: number;
}

/**
 * 入站原始请求体里本网关未认领的顶层键——即转换后会静默消失的字段。
 * 后端读时计算、不落库，仅 include_raw=true 的详情响应返回，故整字段 optional。
 * status 四态见后端 handler/logs/dto.go 的 unclaimedStatus* 常量：
 * ok / raw_not_recorded / style_unsupported / parse_error。
 * fields 仅 status=ok 时有意义（无未认领键时为空数组）；detail 仅 parse_error 给出。
 */
export interface UnclaimedRequestFields {
  status: string;
  fields: string[];
  detail?: string;
}

export interface ChatLog {
  id: number;
  created_at: string;
  name: string;
  provider_model: string;
  provider_name: string;
  status: string;
  style: string;
  user_agent: string;
  remote_ip?: string;
  error: string;
  retry: number;
  proxy_time: number;
  first_chunk_time: number;
  chunk_time: number;
  tps: number;
  chat_io: boolean;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  prompt_tokens_details: PromptTokensDetails;
  completion_tokens_details: CompletionTokensDetails;
  /** usage 归集来源：upstream / passthrough / downstream / missing */
  usage_source: string;
  request_headers?: string;
  request_body?: string;
  raw_request_body?: string;
  response_headers?: string;
  response_body?: string;
  raw_response_body?: string;
  is_virtual_model: boolean;
  has_format_conversion: boolean;
  source_format?: string;
  target_format?: string;
  unclaimed_request_fields?: UnclaimedRequestFields;
}

export interface ChatIO {
  log_id: number;
  input: string;
  of_string?: string | null;
  of_string_array?: string[] | null;
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
