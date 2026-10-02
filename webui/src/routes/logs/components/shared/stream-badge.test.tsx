import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { StreamBadge } from "./stream-badge";

// 共用一览样式档：本实现与 test mock 共源（src = models.ChatLog.stream 的契约）；
// 下游三处（desktop list / mobile list / detail banner）共用断言锁死两态渲染输出与颜色档。

describe("StreamBadge", () => {
  it("流式：渲染 '流式' 文字 + 蓝色文字色档 + data-stream=true", () => {
    const { container } = render(<StreamBadge stream={true} />);
    expect(screen.getByText("流式")).toBeTruthy();
    const badge = container.querySelector('[data-stream="true"]');
    expect(badge).not.toBeNull();
    expect(badge?.className).toMatch(/text-blue-600/);
    expect(badge?.className).not.toMatch(/bg-blue/);
  });

  it("非流式：渲染 '非流式' 文字 + 灰色文字色档 + data-stream=false", () => {
    const { container } = render(<StreamBadge stream={false} />);
    expect(screen.getByText("非流式")).toBeTruthy();
    const badge = container.querySelector('[data-stream="false"]');
    expect(badge).not.toBeNull();
    expect(badge?.className).toMatch(/text-gray-500/);
    expect(badge?.className).not.toMatch(/bg-gray/);
  });

  it("banner 模式：流式带蓝色底色", () => {
    const { container } = render(<StreamBadge stream={true} variant="banner" />);
    const badge = container.querySelector('[data-stream="true"][data-variant="banner"]');
    expect(badge).not.toBeNull();
    expect(badge?.className).toMatch(/bg-blue-50/);
    expect(badge?.className).toMatch(/text-blue-700/);
  });

  it("banner 模式：非流式带灰色底色", () => {
    const { container } = render(<StreamBadge stream={false} variant="banner" />);
    const badge = container.querySelector('[data-stream="false"][data-variant="banner"]');
    expect(badge).not.toBeNull();
    expect(badge?.className).toMatch(/bg-gray-50/);
    expect(badge?.className).toMatch(/text-gray-700/);
  });

  it("图标渲染尺寸与 lucide 一致（size-4 / 16px）", () => {
    const { container: streamContainer } = render(<StreamBadge stream={true} />);
    // SVGElement.className 是 SVGAnimatedString 对象（DOM 类型签名），强转 string 才能 .toMatch
    expect(String(streamContainer.querySelector("svg")?.getAttribute("class") ?? "")).toMatch(/size-4/);
    const { container: nonStreamContainer } = render(<StreamBadge stream={false} />);
    expect(String(nonStreamContainer.querySelector("svg")?.getAttribute("class") ?? "")).toMatch(/size-4/);
  });
});