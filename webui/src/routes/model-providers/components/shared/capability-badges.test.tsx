import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CapabilityBadges } from "./capability-badges";

const MODEL_CAPS = { tool_call: true, structured_output: false, image: true };

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

  it("三项显式禁用且无透传时显示占位文案", () => {
    render(
      <CapabilityBadges toolCall={false} structuredOutput={false} image={false} withHeader={false} />,
    );

    expect(screen.getByText("无")).toBeTruthy();
  });

  it("全显式启用时渲染三个短徽章", () => {
    render(
      <CapabilityBadges toolCall={true} structuredOutput={true} image={true} withHeader={false} />,
    );

    expect(screen.getByText("工具")).toBeTruthy();
    expect(screen.getByText("结构化")).toBeTruthy();
    expect(screen.getByText("视觉")).toBeTruthy();
  });

  // 本列直接显示**最终生效的能力**（继承项按模型真值解析），与后端 COALESCE 同语义。
  // 模型 tool_call=true / structured_output=false / image=true，全继承即「工具 + 视觉」，
  // 不出「结构化」——继承与显式在这列不可见是刻意的：来源属于编辑弹窗，不占列宽。
  it("全继承时按模型真值显示最终能力（工具 + 视觉，无结构化）", () => {
    const { container } = render(
      <CapabilityBadges
        toolCall={null}
        structuredOutput={null}
        image={null}
        withHeader={false}
        modelCaps={MODEL_CAPS}
      />,
    );

    expect(screen.getByText("工具")).toBeTruthy();
    expect(screen.getByText("视觉")).toBeTruthy();
    expect(screen.queryByText("结构化")).toBeNull();
    // 徽章数 = 最终支持数：逐项展开继承文案会让这列溢出压住权重/优先级（已发生过）
    expect(container.firstElementChild?.children).toHaveLength(2);
  });

  it("override 优先于模型真值：模型视觉为 true 时显式 false 仍不显示视觉", () => {
    const { container } = render(
      <CapabilityBadges
        toolCall={null}
        structuredOutput={null}
        image={false}
        withHeader={false}
        modelCaps={MODEL_CAPS}
      />,
    );

    expect(screen.getByText("工具")).toBeTruthy();
    expect(screen.queryByText("视觉")).toBeNull();
    expect(container.firstElementChild?.children).toHaveLength(1);
  });

  it("override 可以比模型更强：模型视觉为 false 时显式 true 仍显示视觉", () => {
    render(
      <CapabilityBadges
        toolCall={null}
        structuredOutput={null}
        image={true}
        withHeader={false}
        modelCaps={MODEL_CAPS}
      />,
    );

    expect(screen.getByText("视觉")).toBeTruthy();
  });

  it("模型真值未加载时降级为「继承模型」徽章，不谎报成「无能力」", () => {
    const { container } = render(
      <CapabilityBadges toolCall={null} structuredOutput={null} image={null} withHeader={false} />,
    );

    expect(screen.getByText("继承模型")).toBeTruthy();
    expect(screen.queryByText("无")).toBeNull();
    expect(container.firstElementChild?.children).toHaveLength(1);
  });
});
