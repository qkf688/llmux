import type { ChatLog } from "@/lib/api";

export type ChatLogExportSectionKey = "basic" | "error" | "performance" | "tokens" | "request" | "response";

export type ChatLogExportSections = Record<ChatLogExportSectionKey, boolean>;

export const DEFAULT_CHAT_LOG_EXPORT_SECTIONS: ChatLogExportSections = {
  basic: true,
  error: true,
  performance: true,
  tokens: true,
  request: true,
  response: true,
};

export type ChatLogExportPayload = {
  log_id?: number;
  created_at?: string;
  model_name?: string;
  provider_name?: string;
  provider_model?: string;
  status?: string;
  style?: string;
  user_agent?: string;
  remote_ip?: string | null;
  retry?: number;
  chat_io?: boolean;
  is_virtual_model?: boolean;
  has_format_conversion?: boolean;
  source_format?: string | null;
  target_format?: string | null;
  error?: string | null;
  performance?: {
    proxy_time: number;
    first_chunk_time: number;
    chunk_time: number;
    tps: number;
  };
  tokens?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
    prompt_tokens_details: ChatLog["prompt_tokens_details"] | null;
    completion_tokens_details: ChatLog["completion_tokens_details"] | null;
    usage_source: string | null;
  };
  request?: {
    headers: string | null;
    body: string | null;
    raw_body: string | null;
  };
  response?: {
    headers: string | null;
    body: string | null;
    raw_body: string | null;
  };
};

function toNullableString(value?: string) {
  const text = typeof value === "string" ? value.trim() : "";
  return text.length > 0 ? text : null;
}

export function buildChatLogExportPayload(log: ChatLog, sections: ChatLogExportSections): ChatLogExportPayload {
  const payload: ChatLogExportPayload = {};

  if (sections.basic) {
    payload.log_id = log.id;
    payload.created_at = log.created_at;
    payload.model_name = log.name;
    payload.provider_name = log.provider_name;
    payload.provider_model = log.provider_model;
    payload.status = log.status;
    payload.style = log.style;
    payload.user_agent = log.user_agent;
    payload.remote_ip = toNullableString(log.remote_ip);
    payload.retry = log.retry;
    payload.chat_io = log.chat_io;
    payload.is_virtual_model = log.is_virtual_model;
    payload.has_format_conversion = log.has_format_conversion;
    payload.source_format = toNullableString(log.source_format);
    payload.target_format = toNullableString(log.target_format);
  }

  if (sections.error) {
    payload.error = toNullableString(log.error);
  }

  if (sections.performance) {
    payload.performance = {
      proxy_time: log.proxy_time,
      first_chunk_time: log.first_chunk_time,
      chunk_time: log.chunk_time,
      tps: log.tps,
    };
  }

  if (sections.tokens) {
    payload.tokens = {
      prompt_tokens: log.prompt_tokens,
      completion_tokens: log.completion_tokens,
      total_tokens: log.total_tokens,
      prompt_tokens_details: log.prompt_tokens_details ?? null,
      completion_tokens_details: log.completion_tokens_details ?? null,
      usage_source: toNullableString(log.usage_source),
    };
  }

  if (sections.request) {
    payload.request = {
      headers: toNullableString(log.request_headers),
      body: toNullableString(log.request_body),
      raw_body: toNullableString(log.raw_request_body),
    };
  }

  if (sections.response) {
    payload.response = {
      headers: toNullableString(log.response_headers),
      body: toNullableString(log.response_body),
      raw_body: toNullableString(log.raw_response_body),
    };
  }

  return payload;
}

export function exportChatLog(
  log: ChatLog,
  sections: ChatLogExportSections = DEFAULT_CHAT_LOG_EXPORT_SECTIONS
) {
  const exportData = buildChatLogExportPayload(log, sections);
  const jsonString = JSON.stringify(exportData, null, 2);
  const blob = new Blob([jsonString], { type: "application/json" });
  const url = URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.href = url;
  link.download = `log-${log.id}-export-${Date.now()}.json`;

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
