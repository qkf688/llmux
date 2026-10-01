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
});
