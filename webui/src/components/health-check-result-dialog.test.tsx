import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { getBatchHealthCheckStatus } from "@/lib/api";
import { createHealthCheckLog } from "@/test/health-check-fixtures";

import { HealthCheckResultDialog } from "./health-check-result-dialog";

vi.mock("@/lib/api", () => ({ getBatchHealthCheckStatus: vi.fn() }));

describe("HealthCheckResultDialog", () => {
  it("未完成单列，四项与总计对得平；最近结果行也按三态显示", async () => {
    vi.mocked(getBatchHealthCheckStatus).mockResolvedValue({
      batch_id: "b1",
      total_count: 5,
      success: 1,
      failed: 1,
      not_measured: 2,
      pending: 1,
      completed: false,
      logs: [createHealthCheckLog(1, "not_measured")],
    });

    render(
      <MemoryRouter>
        <HealthCheckResultDialog open onOpenChange={vi.fn()} batchId="b1" />
      </MemoryRouter>,
    );

    await waitFor(() => expect(screen.getAllByText("未完成")).toHaveLength(2));

    // 统计卡渲染在 Radix 传送门里，故从 document 取；1 + 1 + 2 + 1 = 5 = 总计
    const cards = Array.from(document.querySelectorAll(".grid > div")).map((card) => card.textContent);
    expect(cards).toEqual(["总计5", "成功1", "失败1", "未完成2", "待检测1"]);
  });
});
