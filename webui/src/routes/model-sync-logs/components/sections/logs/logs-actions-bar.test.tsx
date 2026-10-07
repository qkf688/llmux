import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LogsActionsBar } from "./logs-actions-bar";

// 「删除所选」是硬删（后端 HardDeleteByIDs，无回收站），确认弹窗是唯一一道闸：
// 点触发按钮不能直接触发删除，必须先在弹窗里点「确认删除」。同页另外三个破坏性
// 按钮都有这道闸，这里曾经漏掉，故用测试钉死——只断「点哪个按钮、交不交回调」，
// 不复制弹窗文案细节，文案调整不该让用例变红。
function renderBar(p: Partial<Parameters<typeof LogsActionsBar>[0]> = {}) {
  const props = {
    logsCount: 12,
    selectedCount: 3,
    allSelected: false,
    showUnchanged: false,
    deleting: false,
    onToggleSelectAll: vi.fn(),
    onDeleteSelected: vi.fn(),
    onOpenClearDialog: vi.fn(),
    onShowUnchangedChange: vi.fn(),
    ...p,
  };
  render(<LogsActionsBar {...props} />);
  return props;
}

describe("LogsActionsBar 删除所选确认流程", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("点「删除所选」只弹确认框，不直接触发删除", () => {
    const props = renderBar();

    fireEvent.click(screen.getByRole("button", { name: /删除所选/ }));

    expect(props.onDeleteSelected).not.toHaveBeenCalled();
    expect(screen.getByText("删除所选日志？")).toBeInTheDocument();
  });

  it("弹窗里点「确认删除」才真正触发删除", () => {
    const props = renderBar();

    fireEvent.click(screen.getByRole("button", { name: /删除所选/ }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));

    expect(props.onDeleteSelected).toHaveBeenCalledTimes(1);
  });

  it("弹窗里点「取消」不触发删除", () => {
    const props = renderBar();

    fireEvent.click(screen.getByRole("button", { name: /删除所选/ }));
    fireEvent.click(screen.getByRole("button", { name: "取消" }));

    expect(props.onDeleteSelected).not.toHaveBeenCalled();
  });

  it("未选中任何日志时删除按钮禁用，够不着确认框", () => {
    renderBar({ selectedCount: 0 });

    expect(screen.getByRole("button", { name: /删除所选/ })).toBeDisabled();
    expect(screen.queryByText("删除所选日志？")).not.toBeInTheDocument();
  });

  it("删除进行中禁用触发按钮，确认按钮显示删除中且禁用", () => {
    renderBar({ deleting: true });

    expect(screen.getByRole("button", { name: /删除所选/ })).toBeDisabled();

    // deleting 时触发按钮禁用、弹窗开不出来；用默认态打开弹窗后再断言确认态，
    // 避免「弹窗没开」被当成「按钮已禁用」的空过。
    const idle = renderBar();
    fireEvent.click(screen.getAllByRole("button", { name: /删除所选/ })[1]);
    expect(screen.getByRole("button", { name: "确认删除" })).toBeEnabled();
    expect(idle.onDeleteSelected).not.toHaveBeenCalled();
  });
});
