import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { BodyTextBlock } from "./body-text-block";
import { MAX_PREVIEW_CHARS } from "./body-text-policy";

/**
 * 体积闸的契约：超限原文默认不整段进 DOM。
 *
 * 这条闸守的是「日志详情弹窗一打开就卡死」：MB 级单行 JSON 在换行样式的 <pre>
 * 里仅排版就要吃掉十几秒主线程（实测 1.98MB → 15s），页面无响应。
 * 断言只钉**行为**（默认只给头尾、可展开为全文、全文不换行），不钉具体渲染耗时。
 */

/** 中段专属标记：预览只留头尾，它必须既不在预览里、又确实在全文里。 */
const MIDDLE_MARKER = "MIDDLE_ONLY_MARKER";

/** 造一段超限正文：头、中、尾各带一个可断言的标记（填充字符统一用小写 m）。 */
const buildOversizedText = () => {
  const filler = "m".repeat(MAX_PREVIEW_CHARS);
  return `HEAD_MARKER${filler}${MIDDLE_MARKER}${filler}TAIL_MARKER`;
};

const preText = (container: HTMLElement) => container.querySelector("pre")?.textContent ?? "";

describe("BodyTextBlock", () => {
  it("未超限的正文整段渲染，且不出现「查看全文」开关", () => {
    const { container } = render(<BodyTextBlock label="请求头" text='{"a":1}' />);

    expect(preText(container)).toContain('{"a":1}');
    expect(screen.queryByRole("button", { name: "查看全文" })).toBeNull();
  });

  it("超限正文默认只渲染头尾，中段不进入 DOM", () => {
    const { container } = render(<BodyTextBlock label="请求体 - 转换后" text={buildOversizedText()} />);

    const text = preText(container);
    expect(text).toContain("HEAD_MARKER");
    expect(text).toContain("TAIL_MARKER");
    // 中段被省略：这是「不卡死」的关键——整段进 DOM 就是十几秒排版
    expect(text).not.toContain(MIDDLE_MARKER);
    expect(text).toContain("内容过长已截断");
    expect(screen.getByRole("button", { name: "查看全文" })).toBeTruthy();
  });

  it("点「查看全文」后渲染完整正文，且切换为不换行（排版成本的两个数量级差在这里）", () => {
    const { container } = render(<BodyTextBlock label="请求体 - 转换后" text={buildOversizedText()} />);

    fireEvent.click(screen.getByRole("button", { name: "查看全文" }));

    const pre = container.querySelector("pre");
    expect(pre?.textContent).toContain(MIDDLE_MARKER);
    // 不换行：换行会把单行长文本切成上万个行盒，排版成本失控
    expect(pre?.classList.contains("whitespace-pre")).toBe(true);
    expect(pre?.classList.contains("whitespace-pre-wrap")).toBe(false);
    expect(screen.getByRole("button", { name: "收起全文" })).toBeTruthy();
  });

  it("「收起全文」回到头尾预览", () => {
    const { container } = render(<BodyTextBlock label="请求体 - 转换后" text={buildOversizedText()} />);

    fireEvent.click(screen.getByRole("button", { name: "查看全文" }));
    fireEvent.click(screen.getByRole("button", { name: "收起全文" }));

    expect(preText(container)).not.toContain(MIDDLE_MARKER);
    expect(preText(container)).toContain("HEAD_MARKER");
  });
});
