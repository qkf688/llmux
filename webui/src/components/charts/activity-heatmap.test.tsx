import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

import { ActivityHeatmapCard } from "./activity-heatmap";

// jsdom 没有 ResizeObserver：热力图用它观察容器尺寸算格子边长与横向遮罩，给个最小实现
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

vi.stubGlobal("ResizeObserver", ResizeObserverStub);

// 图例格子与热力图共用同一色阶（primary 占比随 level 递增）
function legendSwatchStyles(): string[] {
  const legendRow = screen.getByText("少").parentElement;
  return Array.from(legendRow?.querySelectorAll("div[aria-hidden='true']") ?? []).map(
    (el) => el.getAttribute("style") ?? "",
  );
}

describe("ActivityHeatmapCard 图例色阶", () => {
  it("从「少」到「多」primary 占比单调递增", () => {
    render(<ActivityHeatmapCard data={[]} />);

    const styles = legendSwatchStyles();
    expect(styles).toHaveLength(5);
    // 首格是「无请求」底色，其余为 25% / 45% / 65% / 85% 的档位色。
    // 基底是 --chart-1-strong（不是 --primary）：primary 对卡底只 3.45:1，
    // 浅档位会压到 1.5~2.7:1 与空格融为一体，实测对照见提交说明
    expect(styles[0]).toContain("var(--muted)");
    const percentages = styles.slice(1).map((style) => Number(style.match(/(\d+(?:\.\d+)?)%/)?.[1]));
    expect(percentages).toEqual([25, 45, 65, 85]);
    expect(styles[1]).toContain("--chart-1-strong");
  });
});