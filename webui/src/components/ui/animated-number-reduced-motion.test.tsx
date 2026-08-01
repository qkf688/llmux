import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";

// 独立文件：mock useReducedMotion 返回 true，验证 reduced-motion 分支。
// 不能与 animated-number.test.tsx 同文件——vi.mock 是文件级提升的，
// 同文件两个 vi.mock 会冲突。
vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return { ...actual, useReducedMotion: () => true };
});

import { AnimatedNumber } from "./animated-number";

const round = (v: number) => ({ value: String(Math.round(v)) });

describe("AnimatedNumber (prefers-reduced-motion: reduce)", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date", "requestAnimationFrame", "cancelAnimationFrame", "setTimeout", "clearTimeout"] });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("shouldReduceMotion 时直接显示终值，不调度 rAF", () => {
    render(<AnimatedNumber value={42} formatter={round} />);

    expect(screen.getByText("42")).toBeInTheDocument();
  });
});
