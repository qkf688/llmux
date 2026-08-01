import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";

vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return { ...actual, useReducedMotion: () => false };
});

import { AnimatedNumber } from "./animated-number";

const round = (v: number) => ({ value: String(Math.round(v)) });

describe("AnimatedNumber", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date", "requestAnimationFrame", "cancelAnimationFrame", "setTimeout", "clearTimeout"] });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("初始以 0 起步，动画结束后显示目标值", () => {
    render(<AnimatedNumber value={1000} duration={800} formatter={round} />);

    // 首帧前 display=0
    expect(screen.getByText("0")).toBeInTheDocument();

    // 推进假时钟跑完动画（rAF 由 fake timers 驱动）
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByText("1000")).toBeInTheDocument();
  });

  it("value 变化时从上一次终值继续 lerp（不回跳到 0）", () => {
    const { rerender } = render(
      <AnimatedNumber value={100} duration={800} formatter={round} />
    );

    // 让第一次动画跑完
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByText("100")).toBeInTheDocument();

    // 改值到 200，新动画起点应为 100（上一次终值），而非 0
    rerender(<AnimatedNumber value={200} duration={800} formatter={round} />);

    // 推进到中点（400ms），display 应介于 100 与 200 之间，绝不出现 0
    act(() => {
      vi.advanceTimersByTime(400);
    });
    const mid = Number(screen.getByText(/\d+/).textContent ?? "0");
    expect(mid).toBeGreaterThanOrEqual(100);
    expect(mid).toBeLessThanOrEqual(200);

    // 跑完
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(screen.getByText("200")).toBeInTheDocument();
  });

  it("动画进行中 value 再次变化时，从当前可见值继续（不回跳到旧起点）", () => {
    // 回归测试：fromRef 仅在动画完成时更新，中断时若不保存当前进度，
    // 新动画会从旧起点回跳。修复后 cleanup 把 displayRef 写回 fromRef。
    const { rerender } = render(
      <AnimatedNumber value={100} duration={800} formatter={round} />
    );

    // 推进到中点（400ms），display ≈ 50，动画未完成
    act(() => {
      vi.advanceTimersByTime(400);
    });
    const mid1 = Number(screen.getByText(/\d+/).textContent ?? "0");
    expect(mid1).toBeGreaterThan(0);
    expect(mid1).toBeLessThan(100);

    // 此时改值到 200，新动画应从 mid1（当前可见值）继续，而非从 0 回跳
    rerender(<AnimatedNumber value={200} duration={800} formatter={round} />);

    // 推进一帧，display 应 ≥ mid1（从 mid1 向 200 滚动），绝不出现 0
    act(() => {
      vi.advanceTimersByTime(100);
    });
    const afterInterrupt = Number(screen.getByText(/\d+/).textContent ?? "0");
    expect(afterInterrupt).toBeGreaterThanOrEqual(mid1);

    // 跑完应到 200
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByText("200")).toBeInTheDocument();
  });
});
