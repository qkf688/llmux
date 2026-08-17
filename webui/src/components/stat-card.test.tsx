import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { StatCard, StatGrid } from "./stat-card";

describe("StatGrid", () => {
  it("卡片作为网格的直接子元素渲染", () => {
    const { container } = render(
      <StatGrid>
        <StatCard label="总计" value={3} />
        <StatCard label="成功" value={2} />
      </StatGrid>,
    );
    expect(container.firstElementChild?.childElementCount).toBe(2);
  });

  it("className 可覆写列数", () => {
    const { container } = render(
      <StatGrid className="grid-cols-4">
        <StatCard label="总计" value={1} />
      </StatGrid>,
    );
    expect(container.firstChild).toHaveClass("grid-cols-4");
    expect(container.firstChild).not.toHaveClass("grid-cols-2");
  });
});

describe("StatCard", () => {
  it("渲染指标名与数值，单位紧跟数值", () => {
    const { container } = render(<StatCard label="使用率" value="42.5" unit="%" />);
    expect(screen.getByText("使用率")).toBeInTheDocument();
    // 数值与单位在同一块内，单位是独立小号 span
    const valueBlock = container.querySelector(":scope > div > div:last-child");
    expect(valueBlock?.textContent).toBe("42.5%");
    expect(screen.getByText("%")).toBeInTheDocument();
  });

  it("未传单位与副说明时不渲染多余节点", () => {
    const { container } = render(<StatCard label="总计" value={7} />);
    // 卡内只剩「圆点+label」与「数值」两块
    expect(container.firstElementChild?.childElementCount).toBe(2);
  });

  it("tone 只给圆点上色，数值不着色", () => {
    const { container } = render(<StatCard label="失败" value={1} tone="danger" />);
    expect(container.querySelector("span.rounded-full")).toHaveClass("bg-destructive");
    expect(screen.getByText("1")).not.toHaveClass("text-destructive");
  });

  it("className 传 display 类也不破坏卡内排版", () => {
    // 回归：外层不带 display 类，tailwind-merge 无从顶掉内部 flex（见 ToolbarSearch 同类坑）
    const { container } = render(<StatCard label="总计" value={1} className="hidden" />);
    expect(container.firstChild).toHaveClass("hidden");
    expect(container.querySelector(":scope > div > div")).toHaveClass("flex");
  });
});
