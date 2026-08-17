import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { SelectItem } from "./ui/select";
import { PageToolbar, ToolbarFilter, ToolbarSearch } from "./page-toolbar";

describe("PageToolbar", () => {
  it("内容作为卡片的直接子元素渲染，不额外包 wrapper", () => {
    // 契约：不按 slot 包 wrapper——窄屏隐藏内容时空 wrapper 会被 gap 撑出多余间距
    const { container } = render(
      <PageToolbar>
        <span>筛选区</span>
        <span>搜索区</span>
      </PageToolbar>,
    );
    expect(container.firstElementChild?.childElementCount).toBe(2);
    expect(screen.getByText("筛选区")).toBeInTheDocument();
    expect(screen.getByText("搜索区")).toBeInTheDocument();
  });

  it("应用自定义 className 到外层容器", () => {
    const { container } = render(
      <PageToolbar className="custom-cls">
        <span>内容</span>
      </PageToolbar>,
    );
    expect(container.firstChild).toHaveClass("custom-cls");
  });
});

describe("ToolbarFilter", () => {
  it("渲染标签，并把标签用作触发器的无障碍名称", () => {
    render(
      <ToolbarFilter label="模型名称" value="all" onValueChange={() => {}} placeholder="选择模型">
        <SelectItem value="all">全部</SelectItem>
      </ToolbarFilter>,
    );
    expect(screen.getByText("模型名称")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "模型名称" })).toBeInTheDocument();
  });

  it("className 可覆写显隐（窄屏隐藏场景）", () => {
    const { container } = render(
      <ToolbarFilter label="状态" value="all" onValueChange={() => {}} className="hidden sm:flex">
        <SelectItem value="all">全部</SelectItem>
      </ToolbarFilter>,
    );
    expect(container.firstChild).toHaveClass("hidden");
    expect(container.firstChild).not.toHaveClass("flex");
  });
});

describe("ToolbarSearch", () => {
  it("回传输入值而非事件对象", () => {
    const onChange = vi.fn();
    render(<ToolbarSearch value="" onChange={onChange} ariaLabel="搜索提供商名称" />);
    fireEvent.change(screen.getByRole("textbox", { name: "搜索提供商名称" }), {
      target: { value: "gpt" },
    });
    expect(onChange).toHaveBeenCalledWith("gpt");
  });

  it("className 传 display 类也不破坏内部图标+输入的 flex 排列", () => {
    // 回归：单层结构时 `sm:block` 被 tailwind-merge 判为与 flex 冲突，图标与输入错行、文字溢出卡片
    const { container } = render(
      <ToolbarSearch value="" onChange={() => {}} className="hidden sm:block" />,
    );
    expect(container.firstChild).toHaveClass("hidden");
    expect(container.querySelector(":scope > div > div")).toHaveClass("flex");
  });
});
