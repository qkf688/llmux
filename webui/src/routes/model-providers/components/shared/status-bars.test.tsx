import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBars } from "./status-bars";

function renderBars(props: Parameters<typeof StatusBars>[0]) {
  const { container } = render(<StatusBars {...props} />);
  return Array.from(container.firstElementChild?.children ?? []) as HTMLElement[];
}

describe("StatusBars", () => {
  it("未完成照旧占一格，给第三态样式与固定 title，前两态配色文案不变", () => {
    const bars = renderBars({
      bars: ["success", "error", "not_measured"],
      successClassName: "bg-success",
      failClassName: "bg-warning",
      showTitle: true,
      successTitle: "健康检测成功",
      failTitle: "健康检测失败",
    });

    expect(bars).toHaveLength(3);
    expect(bars[0].className).toContain("bg-success");
    expect(bars[1].className).toContain("bg-warning");
    // 第三态不与失败同色（本机没测成不是供应商的结论）
    expect(bars[2].className).not.toContain("bg-warning");
    expect(bars[2].className).toContain("bg-muted-foreground/40");
    expect(bars[2].getAttribute("title")).toBe("未完成（本机原因）");
  });

  it("布尔来源在调用处映射后同样按两态着色（组件只认三态词汇）", () => {
    const bars = renderBars({
      // 聊天成败这一路在调用方显式映射：true → success、false → error
      bars: ["success", "error"],
      successClassName: "bg-success",
      failClassName: "bg-destructive",
    });

    expect(bars.map((bar) => bar.className)).toEqual([
      expect.stringContaining("bg-success"),
      expect.stringContaining("bg-destructive"),
    ]);
  });
});
