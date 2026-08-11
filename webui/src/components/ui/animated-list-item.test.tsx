import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// mock useReducedMotion 返回 true，验证退化分支：渲染成普通 div、不挂 motion。
vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return { ...actual, useReducedMotion: () => true };
});

import { AnimatedListItem } from "./animated-list-item";

describe("AnimatedListItem (prefers-reduced-motion: reduce)", () => {
  it("shouldReduceMotion 时渲染普通 div，透传 className 与 children", () => {
    render(
      <AnimatedListItem className="my-row">
        <span>row content</span>
      </AnimatedListItem>,
    );

    const content = screen.getByText("row content");
    expect(content).toBeInTheDocument();
    // 退化分支挂 className 的容器不带 motion 内联 style（无残留 transform）
    const container = content.parentElement;
    expect(container).toHaveClass("my-row");
    expect(container?.getAttribute("style") ?? "").toBe("");
  });
});
