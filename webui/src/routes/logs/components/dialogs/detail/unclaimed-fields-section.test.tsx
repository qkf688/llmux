import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";

import type { ChatLog, MismatchedRequestFields, UnclaimedRequestFields } from "@/lib/api";
import { MismatchedFieldsSection, UnclaimedFieldsSection } from "./unclaimed-fields-section";

/**
 * 用具名参数而非位置参数：两类诊断字段独立缺席/独立赋值，
 * `buildLog(undefined, "openai", {...})` 这种位置写法读不出在测哪一块。
 */
const buildLog = ({
  unclaimed,
  mismatched,
  style = "openai",
}: {
  unclaimed?: UnclaimedRequestFields;
  mismatched?: MismatchedRequestFields;
  style?: string;
} = {}): ChatLog => ({
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
  mismatched_request_fields: mismatched,
});

describe("UnclaimedFieldsSection", () => {
  it("字段缺席时整块不渲染", () => {
    // include_raw=false 的列表行不带该键，此时无任何结论可说
    const { container } = render(<UnclaimedFieldsSection log={buildLog()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("ok 且有未认领键时列出键名与计数", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({ unclaimed: { status: "ok", fields: ["foo_bar", "baz"] } })}
      />,
    );

    expect(screen.getByText("foo_bar")).toBeInTheDocument();
    expect(screen.getByText("baz")).toBeInTheDocument();
    expect(screen.getByText(/发现 2 个网关未识别的请求字段/)).toBeInTheDocument();
  });

  it("ok 且无未认领键时说明是「已检测」而非「查不了」", () => {
    render(<UnclaimedFieldsSection log={buildLog({ unclaimed: { status: "ok", fields: [] } })} />);

    // 必须明确表达检测已完成，否则与 raw_not_recorded 无从区分
    expect(screen.getByText(/已检测/)).toBeInTheDocument();
  });

  it("raw_not_recorded 说明是原始体未记录、无从检测，不渲染成「无问题」", () => {
    render(
      <UnclaimedFieldsSection log={buildLog({ unclaimed: { status: "raw_not_recorded", fields: [] } })} />,
    );

    expect(screen.getByText(/未记录原始请求体/)).toBeInTheDocument();
    expect(screen.queryByText(/已检测/)).not.toBeInTheDocument();
  });

  it("style_unsupported 写明是该入站协议的检测能力缺口而非请求出错", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({ unclaimed: { status: "style_unsupported", fields: [] }, style: "anthropic" })}
      />,
    );

    expect(screen.getByText(/anthropic/)).toBeInTheDocument();
    expect(screen.getByText(/尚未实现/)).toBeInTheDocument();
    expect(screen.queryByText(/已检测/)).not.toBeInTheDocument();
  });

  it("parse_error 展示后端给出的 detail", () => {
    render(
      <UnclaimedFieldsSection
        log={buildLog({
          unclaimed: {
            status: "parse_error",
            fields: [],
            detail: "unexpected end of JSON input",
          },
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
      const { container, unmount } = render(<UnclaimedFieldsSection log={buildLog({ unclaimed })} />);
      const text = container.textContent ?? "";
      unmount();
      return text;
    });

    expect(new Set(texts).size).toBe(4);
  });

  it("有未认领键时给出语义边界说明，无从检测的态不给", () => {
    const withFields = render(
      <UnclaimedFieldsSection log={buildLog({ unclaimed: { status: "ok", fields: ["foo"] } })} />,
    );
    // 「网关不认领」≠「转换丢失」，且只看顶层键——不说清会被读成「转换已无问题」
    // （说明收进默认折叠的 <details>，内容仍在 DOM 中可查）
    expect(screen.getByText(/不等于/)).toBeInTheDocument();
    expect(screen.getByText(/这项检测是什么意思/)).toBeInTheDocument();
    withFields.unmount();

    render(<UnclaimedFieldsSection log={buildLog({ unclaimed: { status: "raw_not_recorded", fields: [] } })} />);
    expect(screen.queryByText(/不等于/)).not.toBeInTheDocument();
  });
});

describe("MismatchedFieldsSection", () => {
  it("字段缺席时整块不渲染", () => {
    const { container } = render(<MismatchedFieldsSection log={buildLog()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("只读 mismatched_request_fields，不受未认领字段影响", () => {
    // 两块共用一个内部组件，取错字段会让两块显示同一份结论
    const { container } = render(
      <MismatchedFieldsSection log={buildLog({ unclaimed: { status: "ok", fields: ["foo_bar"] } })} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("ok 且有类型不符键时列出键名并说明请求照常成功", () => {
    render(
      <MismatchedFieldsSection log={buildLog({ mismatched: { status: "ok", fields: ["temperature"] } })} />,
    );

    expect(screen.getByText("temperature")).toBeInTheDocument();
    expect(screen.getByText(/1 个字段因值类型不符被忽略/)).toBeInTheDocument();
    // 用户最需要知道的是「没报错但参数没生效」，否则不会去查客户端
    expect(screen.getByText(/请求照常成功/)).toBeInTheDocument();
  });

  it("ok 且无类型不符键时说明是「已检测」而非「查不了」", () => {
    render(<MismatchedFieldsSection log={buildLog({ mismatched: { status: "ok", fields: [] } })} />);

    expect(screen.getByText(/已检测/)).toBeInTheDocument();
  });

  it("style_unsupported 表述为「不适用」而非能力缺口", () => {
    // 与未认领的同名状态语义相反：openai-res 是直接报错、无静默丢弃，
    // 若沿用「尚未实现」会被误读成待补的 TODO
    render(
      <MismatchedFieldsSection
        log={buildLog({ mismatched: { status: "style_unsupported", fields: [] }, style: "openai-res" })}
      />,
    );

    expect(screen.getByText(/不适用/)).toBeInTheDocument();
    expect(screen.getByText(/openai-res/)).toBeInTheDocument();
    expect(screen.queryByText(/尚未实现/)).not.toBeInTheDocument();
  });

  it("四态文案互不相同", () => {
    const texts = (
      [
        { status: "ok", fields: [] },
        { status: "raw_not_recorded", fields: [] },
        { status: "style_unsupported", fields: [] },
        { status: "parse_error", fields: [], detail: "bad body" },
      ] satisfies MismatchedRequestFields[]
    ).map((mismatched) => {
      const { container, unmount } = render(<MismatchedFieldsSection log={buildLog({ mismatched })} />);
      const text = container.textContent ?? "";
      unmount();
      return text;
    });

    expect(new Set(texts).size).toBe(4);
  });

  it("与未认领块的同态文案不重合，避免两块结论被混读", () => {
    const readText = (element: ReactElement) => {
      const { container, unmount } = render(element);
      const text = container.textContent ?? "";
      unmount();
      return text;
    };

    const unclaimedText = readText(
      <UnclaimedFieldsSection log={buildLog({ unclaimed: { status: "ok", fields: ["foo"] } })} />,
    );
    const mismatchedText = readText(
      <MismatchedFieldsSection log={buildLog({ mismatched: { status: "ok", fields: ["foo"] } })} />,
    );

    expect(unclaimedText).not.toBe(mismatchedText);
  });
});
