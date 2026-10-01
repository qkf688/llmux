import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";

import { DesktopAssociationTable } from "./desktop-association-table";

// 空数据即可覆盖表头与 colgroup：这两个才是列宽契约的载体，行内容与列宽无关。
// App 根挂 TooltipProvider（「状态」列头的刷新按钮依赖它）——测试里同构包一层。
function renderTable() {
  return render(
    <TooltipProvider>
      <DesktopAssociationTable
        associations={[]}
        providers={[]}
        selectedAssociationIds={[]}
        isAllSelected={false}
        isPartialSelected={false}
        providerStatus={{}}
        healthStatus={{}}
        statusUpdating={{}}
        associationTestResults={{}}
        deleteId={null}
        onSelectAll={vi.fn()}
        onSelectOne={vi.fn()}
        onRefreshStatus={vi.fn()}
        onToggleStatus={vi.fn()}
        onEdit={vi.fn()}
        onOpenDelete={vi.fn()}
        onDeleteDialogChange={vi.fn()}
        onDeleteConfirm={vi.fn()}
        onTest={vi.fn()}
      />
    </TooltipProvider>,
  );
}

describe("DesktopAssociationTable", () => {
  // 之前是 auto 布局：长模型名/供应商名会挤窄「能力」「操作」列，触发内部 flex-wrap 换行，
  // 同一张表里行高差 2 倍以上。固定布局 + colgroup 是这条不回退的保证。
  it("用固定布局渲染，且每列宽度由 colgroup 声明", () => {
    const { container } = renderTable();

    const table = container.querySelector("table");
    expect(table?.className).toContain("table-fixed");

    const cols = container.querySelectorAll("colgroup col");
    const heads = container.querySelectorAll("thead th");
    expect(cols).toHaveLength(heads.length);
    // colgroup 至少要覆盖除「提供商模型」以外的每一列，否则列宽又回到内容驱动
    expect(container.querySelectorAll("colgroup col[class]").length).toBe(heads.length - 1);
  });
});
