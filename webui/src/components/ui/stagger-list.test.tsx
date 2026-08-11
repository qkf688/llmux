import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// 用可变开关驱动 mock：同一文件内既验证 reduced 退化分支，也验证 motion 分支的 resetKey 契约。
let reduceMotion = true;
vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return { ...actual, useReducedMotion: () => reduceMotion };
});

import { StaggerItem, StaggerList } from "./stagger-list";

describe("StaggerList / StaggerItem (prefers-reduced-motion: reduce)", () => {
  it("shouldReduceMotion 时渲染普通 div，透传 className 与 children", () => {
    reduceMotion = true;
    render(
      <StaggerList className="my-grid">
        <StaggerItem className="my-cell">
          <span>cell content</span>
        </StaggerItem>
      </StaggerList>,
    );

    const content = screen.getByText("cell content");
    const item = content.parentElement;
    expect(item).toHaveClass("my-cell");
    // 退化分支不挂 motion，元素上不应留内联 style（无残留 transform / filter）
    expect(item?.getAttribute("style") ?? "").toBe("");
    expect(item?.parentElement).toHaveClass("my-grid");
    expect(item?.parentElement?.getAttribute("style") ?? "").toBe("");
  });
});

describe("StaggerList resetKey", () => {
  function MountProbe({ onMount }: { onMount: () => void }) {
    React.useEffect(onMount, [onMount]);
    return <span>probe</span>;
  }

  it("resetKey 变化时重挂载容器以重播入场；不变则保持挂载", () => {
    reduceMotion = false;
    const onMount = vi.fn();

    const { rerender } = render(
      <StaggerList resetKey={1}>
        <MountProbe onMount={onMount} />
      </StaggerList>,
    );
    expect(onMount).toHaveBeenCalledTimes(1);

    rerender(
      <StaggerList resetKey={1}>
        <MountProbe onMount={onMount} />
      </StaggerList>,
    );
    expect(onMount).toHaveBeenCalledTimes(1);

    rerender(
      <StaggerList resetKey={2}>
        <MountProbe onMount={onMount} />
      </StaggerList>,
    );
    expect(onMount).toHaveBeenCalledTimes(2);
  });
});
