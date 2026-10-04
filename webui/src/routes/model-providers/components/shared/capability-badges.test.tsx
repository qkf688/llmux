import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CapabilityBadges } from "./capability-badges";

function renderBadges(props: Parameters<typeof CapabilityBadges>[0]) {
  const { container } = render(<CapabilityBadges {...props} />);
  return container.firstElementChild as HTMLElement;
}

describe("CapabilityBadges", () => {
  it("默认允许换行，供宽度自由的容器（移动端卡片）使用", () => {
    const cluster = renderBadges({
      toolCall: true,
      structuredOutput: true,
      image: true,
      withHeader: true,
    });

    expect(cluster.className).toContain("flex-wrap");
    expect(cluster.className).not.toContain("flex-nowrap");
  });

  it("nowrap 时徽章不换行：列宽固定的表格里 wrap 会把行高撑成两倍", () => {
    const cluster = renderBadges({
      nowrap: true,
      toolCall: true,
      structuredOutput: true,
      image: true,
      withHeader: true,
    });

    expect(cluster.className).toContain("flex-nowrap");
    expect(cluster.className).not.toContain("flex-wrap");
  });

  it("无能力时显示占位文案，不渲染空容器", () => {
    render(
      <CapabilityBadges toolCall={false} structuredOutput={false} image={false} withHeader={false} />,
    );

    expect(screen.getByText("无")).toBeTruthy();
  });

  // 继承态（null）必须与「显式关闭」（false）视觉可区分：
  // 两者最终能力可能不同（继承的是模型真值），压成同一个渲染会让用户
  // 误以为该链路不支持，从而重复去关联上逐条标注——正是本次改动要消灭的动作。
  it("继承态与显式关闭渲染不同：继承带 modelCaps 时显示实际取值", () => {
    render(
      <CapabilityBadges
        toolCall={null}
        structuredOutput={false}
        image={null}
        withHeader={false}
        modelCaps={{ tool_call: true, structured_output: false, image: true }}
      />,
    );

    expect(screen.getByText("工具·继承（启用）")).toBeTruthy();
    expect(screen.getByText("视觉·继承（启用）")).toBeTruthy();
    // 显式 false 不渲染徽章，也不会与继承态混成同一段文案
    expect(screen.queryByText("结构化")).toBeNull();
  });

  it("无 modelCaps 时继承态仍标注「继承」，不假装成已启用", () => {
    render(
      <CapabilityBadges toolCall={null} structuredOutput={null} image={null} withHeader={false} />,
    );

    expect(screen.getByText("工具·继承")).toBeTruthy();
    expect(screen.getByText("结构化·继承")).toBeTruthy();
    expect(screen.getByText("视觉·继承")).toBeTruthy();
    expect(screen.queryByText("无")).toBeNull();
  });
});
