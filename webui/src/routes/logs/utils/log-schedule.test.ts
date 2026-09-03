import { describe, expect, it } from "vitest";

import type { ChatLog } from "@/lib/api";
import { splitLogSchedule } from "./log-schedule";

/** 只填调度四字段，其余字段与派生无关；基座完整 + Partial spread——返回注解代替
 *  `as` 断言：未来 ChatLog 新增必填字段时漏更基座会在此编译报错而非静默放行 */
const buildLog = (schedule: Partial<ChatLog>): ChatLog => ({
  id: 1,
  created_at: "2026-09-03T00:00:00Z",
  name: "gpt-x",
  provider_model: "gpt-x-1",
  provider_name: "openai",
  status: "success",
  style: "openai",
  user_agent: "curl/8",
  error: "",
  retry: 0,
  proxy_time: 1,
  first_chunk_time: 1,
  chunk_time: 1,
  tps: 1,
  chat_io: false,
  prompt_tokens: 0,
  completion_tokens: 0,
  total_tokens: 0,
  prompt_tokens_details: { cached_tokens: 0, audio_tokens: 0 },
  completion_tokens_details: { reasoning_tokens: 0, audio_tokens: 0 },
  usage_source: "upstream",
  is_virtual_model: false,
  has_format_conversion: false,
  ...schedule,
});

describe("splitLogSchedule", () => {
  it.each([
    {
      case: "完整态：两层均有值（S3-3 起新链路行）",
      log: buildLog({
        endpoint_protocol: "openai",
        endpoint_url: "https://api.example.com/v1",
        key_group_name: "低价组",
        credential_note: "#a1b2c3d4",
      }),
      want: {
        hasAny: true,
        endpointLine: "openai · https://api.example.com/v1",
        credentialLine: "低价组 · #a1b2c3d4",
      },
    },
    {
      case: "空态：四字段全空（S3-3 之前的存量行）",
      log: buildLog({}),
      want: { hasAny: false, endpointLine: "", credentialLine: "" },
    },
    {
      case: "空字段显式空串与键缺失（列表行可选键）等价",
      log: buildLog({
        endpoint_protocol: "",
        endpoint_url: "",
        key_group_name: "",
        credential_note: "",
      }),
      want: { hasAny: false, endpointLine: "", credentialLine: "" },
    },
    {
      case: "部分态：端点层缺席、凭据层完整",
      log: buildLog({ key_group_name: "低价组", credential_note: "prod-key" }),
      want: { hasAny: true, endpointLine: "", credentialLine: "低价组 · prod-key" },
    },
    {
      case: "部分态：端点层完整、凭据层缺席（分组未命名且凭据标识空）",
      log: buildLog({ endpoint_protocol: "anthropic", endpoint_url: "https://ep.example" }),
      want: { hasAny: true, endpointLine: "anthropic · https://ep.example", credentialLine: "" },
    },
    {
      case: "层内单字段：只余协议（URL 继承链空）",
      log: buildLog({ endpoint_protocol: "openai" }),
      want: { hasAny: true, endpointLine: "openai", credentialLine: "" },
    },
    {
      case: "层内单字段：只余分组名",
      log: buildLog({ key_group_name: "默认组" }),
      want: { hasAny: true, endpointLine: "", credentialLine: "默认组" },
    },
  ])("$case", ({ log, want }) => {
    expect(splitLogSchedule(log)).toEqual(want);
  });
});
