import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { LoadingState } from "./loading-state";

describe("LoadingState", () => {
  it("渲染默认文案与 spinner", () => {
    render(<LoadingState />);
    expect(screen.getByText("加载中...")).toBeInTheDocument();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("支持自定义文案", () => {
    render(<LoadingState text="测试中..." />);
    expect(screen.getByText("测试中...")).toBeInTheDocument();
  });

  it("text 为空字符串时不渲染文案节点", () => {
    const { container } = render(<LoadingState text="" />);
    // spinner 仍存在
    expect(screen.getByRole("status")).toBeInTheDocument();
    // 不存在默认文案
    expect(screen.queryByText("加载中...")).not.toBeInTheDocument();
    // 容器内无 span 文本节点（spinner 是 svg，无 span）
    expect(container.querySelector("span")).toBeNull();
  });

  it("应用自定义 className 到外层容器", () => {
    const { container } = render(<LoadingState className="custom-cls" />);
    expect(container.firstChild).toHaveClass("custom-cls");
  });
});
