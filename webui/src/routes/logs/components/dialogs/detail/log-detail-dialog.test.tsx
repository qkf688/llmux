import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LogDetailDialog } from "./log-detail-dialog";
import type { ChatLog } from "@/lib/api";

// Stage 17 契约（前端侧）：日志详情弹窗展示带来源前缀的请求编号（gw:/in:）；
// 存量行 request_id 空/缺失时显示占位而非崩溃或误显。
// mock 掉详情拉取：fixture 自带 raw 字段，needsLogDetail 判 false 不发请求。
vi.mock("@/lib/api", () => ({
  getLogDetail: vi.fn(),
}));

const makeLog = (p: Partial<ChatLog>): ChatLog => ({
  id: 1,
  created_at: "2026-09-21T05:00:00Z",
  name: "gpt-x",
  provider_model: "gpt-x-1",
  provider_name: "prov",
  status: "success",
  style: "openai",
  user_agent: "cli/1.0",
  error: "",
  retry: 0,
  proxy_time: 0,
  first_chunk_time: 0,
  chunk_time: 0,
  tps: 0,
  chat_io: false,
  prompt_tokens: 0,
  completion_tokens: 0,
  total_tokens: 0,
  prompt_tokens_details: { cached_tokens: 0, audio_tokens: 0 },
  completion_tokens_details: { reasoning_tokens: 0, audio_tokens: 0 },
  usage_source: "upstream",
  is_virtual_model: false,
  has_format_conversion: false,
  // 后端键恒在：存量行/内部路径为空串 = 无编号（DTO 无 omitempty）
  request_id: "",
  // 自带 raw 字段：needsLogDetail 判 false，组件不再拉详情（隔离 API 依赖）
  request_headers: "{}",
  request_body: "{}",
  raw_request_body: "{}",
  response_headers: "{}",
  response_body: "{}",
  raw_response_body: "{}",
  ...p,
});

function renderDialog(log: ChatLog) {
  render(<LogDetailDialog open log={log} onOpenChange={() => {}} onExportLog={() => {}} />);
}

describe("LogDetailDialog 请求编号展示（Stage 17）", () => {
  it("展示带来源前缀的请求编号", () => {
    renderDialog(makeLog({ request_id: "in:trace-0921" }));
    expect(screen.getByText("请求编号")).toBeTruthy();
    expect(screen.getByText("in:trace-0921")).toBeTruthy();
  });

  it("自生成前缀同样展示", () => {
    renderDialog(makeLog({ request_id: "gw:0b70b5bcda3e" }));
    expect(screen.getByText("gw:0b70b5bcda3e")).toBeTruthy();
  });

  it("存量行无编号（空串）时显示占位，不崩溃", () => {
    // 后端契约：键恒在，存量行值为空串——缺键不是生产形态
    renderDialog(makeLog({ request_id: "" }));
    expect(screen.getByText("请求编号")).toBeTruthy();
    expect(screen.queryByText(/^gw:/)).toBeNull();
    expect(screen.queryByText(/^in:/)).toBeNull();
    expect(screen.getAllByText("-").length).toBeGreaterThan(0);
  });
});
