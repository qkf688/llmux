import { describe, expect, it } from "vitest";
import type { ChatLog } from "@/lib/api";
import { buildChatLogExportPayload, DEFAULT_CHAT_LOG_EXPORT_SECTIONS } from "./export-log";

const buildBaseLog = (): ChatLog => ({
  id: 123,
  created_at: "2026-04-05T00:00:00Z",
  name: "gpt-x",
  provider_model: "gpt-x-1",
  provider_name: "openai",
  status: "success",
  style: "openai",
  user_agent: "curl/8",
  remote_ip: "127.0.0.1",
  error: "",
  retry: 0,
  proxy_time: 1000,
  first_chunk_time: 2000,
  chunk_time: 3000,
  tps: 12.34,
  chat_io: true,
  prompt_tokens: 10,
  completion_tokens: 20,
  total_tokens: 30,
  prompt_tokens_details: { cached_tokens: 1, audio_tokens: 0 },
  completion_tokens_details: { reasoning_tokens: 2, audio_tokens: 0 },
  usage_source: "upstream",
  request_headers: "{\"x\": \"y\"}",
  request_body: "{\"input\": \"hi\"}",
  raw_request_body: "{\"raw\": \"client\"}",
  response_headers: "{\"ok\": true}",
  response_body: "{\"output\": \"hello\"}",
  raw_response_body: "{\"output\": \"hello\"}",
  is_virtual_model: false,
  has_format_conversion: false,
});

describe("buildChatLogExportPayload", () => {
  it("exports all sections by default", () => {
    const payload = buildChatLogExportPayload(buildBaseLog(), DEFAULT_CHAT_LOG_EXPORT_SECTIONS);

    expect(payload).toMatchObject({
      log_id: 123,
      model_name: "gpt-x",
      performance: {
        proxy_time: 1000,
        first_chunk_time: 2000,
        chunk_time: 3000,
        tps: 12.34,
      },
      tokens: {
        prompt_tokens: 10,
        completion_tokens: 20,
        total_tokens: 30,
        prompt_tokens_details: { cached_tokens: 1, audio_tokens: 0 },
        completion_tokens_details: { reasoning_tokens: 2, audio_tokens: 0 },
        usage_source: "upstream",
      },
      request: {
        headers: "{\"x\": \"y\"}",
        body: "{\"input\": \"hi\"}",
        raw_body: "{\"raw\": \"client\"}",
      },
      response: {
        headers: "{\"ok\": true}",
        body: "{\"output\": \"hello\"}",
        raw_body: "{\"output\": \"hello\"}",
      },
    });
  });

  it("omits unchecked sections", () => {
    const payload = buildChatLogExportPayload(buildBaseLog(), {
      basic: false,
      error: false,
      performance: false,
      tokens: false,
      request: true,
      response: false,
    });

    expect(payload).toEqual({
      request: {
        headers: "{\"x\": \"y\"}",
        body: "{\"input\": \"hi\"}",
        raw_body: "{\"raw\": \"client\"}",
      },
    });
  });
});

