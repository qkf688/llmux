import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import type { ChatLog, UnclaimedRequestFields } from "@/lib/api";
import { UnclaimedFieldsSection } from "./unclaimed-fields-section";

const buildLog = (
  unclaimed?: UnclaimedRequestFields,
  style = "openai",
): ChatLog => ({
  id: 1,
  created_at: "2026-08-19T00:00:00Z",
  name: "gpt-x",
  provider_model: "gpt-x-1",
  provider_name: "openai",
  status: "success",
  style,
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
  unclaimed_request_fields: unclaimed,
});

describe("UnclaimedFieldsSection", () => {
  it("字段缺席时整块不渲染", () => {
    // include_raw=false 的列表行不带该键，此时无任何结论可说
    const { container } = render(<UnclaimedFieldsSection log={buildLog(undefined)} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("ok 且有未认领键时列出键名与计数", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({ status: "ok", fields: ["foo_bar", "baz"] })}
      />,
    );

    expect(screen.getByText("foo_bar")).toBeInTheDocument();
    expect(screen.getByText("baz")).toBeInTheDocument();
    expect(screen.getByText(/未解析的顶层键 \(2\)/)).toBeInTheDocument();
  });

  it("ok 且无未认领键时说明是「已检测」而非「查不了」", () => {
    render(<UnclaimedFieldsSection log={buildLog({ status: "ok", fields: [] })} />);

    // 必须明确表达检测已完成，否则与 raw_not_recorded 无从区分
    expect(screen.getByText(/已完成检测/)).toBeInTheDocument();
  });

  it("raw_not_recorded 说明是原始体未记录、无从检测，不渲染成「无问题」", () => {
    render(
      <UnclaimedFieldsSection log={buildLog({ status: "raw_not_recorded", fields: [] })} />,
    );

    expect(screen.getByText(/未记录原始请求体/)).toBeInTheDocument();
    expect(screen.queryByText(/已完成检测/)).not.toBeInTheDocument();
  });

  it("style_unsupported 写明是该入站协议的检测能力缺口而非请求出错", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({ status: "style_unsupported", fields: [] }, "anthropic")}
      />,
    );

    expect(screen.getByText(/anthropic/)).toBeInTheDocument();
    expect(screen.getByText(/尚未实现/)).toBeInTheDocument();
    expect(screen.queryByText(/已完成检测/)).not.toBeInTheDocument();
  });

  it("parse_error 展示后端给出的 detail", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({
          status: "parse_error",
          fields: [],
          detail: "unexpected end of JSON input",
        })}
      />,
    );

    expect(screen.getByText(/unexpected end of JSON input/)).toBeInTheDocument();
  });

  it("四态文案互不相同", () => {
    // 钉死「把四态压成两态」的回归：任意两态文案相同都会让某一态骗人
    const texts = (
      [
        { status: "ok", fields: [] },
        { status: "raw_not_recorded", fields: [] },
        { status: "style_unsupported", fields: [] },
        { status: "parse_error", fields: [], detail: "bad body" },
      ] satisfies UnclaimedRequestFields[]
    ).map((unclaimed) => {
      const { container, unmount } = render(<UnclaimedFieldsSection log={buildLog(unclaimed)} />);
      const text = container.textContent ?? "";
      unmount();
      return text;
    });

    expect(new Set(texts).size).toBe(4);
  });

  it("有未认领键时给出语义边界说明，无从检测的态不给", () => {
    const withFields = render(
      <UnclaimedFieldsSection log={buildLog({ status: "ok", fields: ["foo"] })} />,
    );
    // 「网关不认领」≠「转换丢失」，且只看顶层键——不说清会被读成「转换已无问题」
    expect(screen.getByText(/不等于/)).toBeInTheDocument();
    expect(screen.getByText(/顶层键/)).toBeInTheDocument();
    withFields.unmount();

    render(<UnclaimedFieldsSection log={buildLog({ status: "raw_not_recorded", fields: [] })} />);
    expect(screen.queryByText(/不等于/)).not.toBeInTheDocument();
  });
});
