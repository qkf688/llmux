import { apiRequest } from "../../core/client";

export interface PromptTokensDetails {
  cached_tokens: number;
  audio_tokens: number;
}

export interface CompletionTokensDetails {
  reasoning_tokens: number;
  /** true = 上游明确报告了 reasoning 拆分（0 也是报告）；false/缺失 = 未报告，推理应展示「未知」 */
  reasoning_tokens_known?: boolean;
  audio_tokens: number;
}

/**
 * 检测状态四态，逐字对应后端 handler/logs/dto.go 的 unclaimedStatus* 常量。
 * 收窄成字面量联合而非 string，是为了让消费方的四态分支能拿到穷尽性检查——
 * 后端新增第五态时，未补分支的地方会编译报错而不是静默走进兜底文案。
 *
 * unclaimed / mismatched 两类诊断共用同一套状态枚举（后端也共用一个 struct）。
 */
export type UnclaimedStatus = "ok" | "raw_not_recorded" | "style_unsupported" | "parse_error";

/**
 * 入站原始请求体里本网关未认领的顶层键——即转换后会静默消失的字段。
 * 后端读时计算、不落库，仅 include_raw=true 的详情响应返回，故整字段 optional。
 * fields 仅 status=ok 时有意义（无未认领键时为空数组）；detail 仅 parse_error 给出。
 */
export interface UnclaimedRequestFields {
  status: UnclaimedStatus;
  fields: string[];
  detail?: string;
}

/**
 * 入站请求体里**被认领、但值类型不匹配而被静默丢弃**的顶层键——键名对得上，但值类型
 * 与入站 DTO 不符，被宽容容器当成「没传」（如 `temperature:"0.5"` 会 200 通过但不生效）。
 *
 * 与 UnclaimedRequestFields 形状相同（共用后端 struct 与状态枚举），但语义不同：前者是
 * 网关没实现该字段，后者是客户端传错类型。分成两个 interface 而非复用同名，是为了让调用
 * 处的类型意图自解释。openai-res 因入站 DTO 用裸类型/指针，类型不对时整条请求解析失败、
 * 无静默可报，故它的 status 恒为 style_unsupported。
 */
export interface MismatchedRequestFields {
  status: UnclaimedStatus;
  fields: string[];
  detail?: string;
}

export interface ChatLog {
  id: number;
  created_at: string;
  name: string;
  provider_model: string;
  provider_name: string;
  /** 调度明细（S0 原型预留；后端 S3 选路改造后填充）：命中的协议端点 */
  endpoint_protocol?: string;
  /** 命中的端点 URL */
  endpoint_url?: string;
  /** 命中的凭据分组 */
  key_group_name?: string;
  /** 命中的凭据（note / 脱敏标识） */
  credential_note?: string;
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
  mismatched_request_fields?: MismatchedRequestFields;
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
