import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { createHealthCheckLog } from "@/test/health-check-fixtures";

import { HealthCheckLogsDesktopTable } from "./health-check-logs-desktop-table";

describe("HealthCheckLogsDesktopTable", () => {
  it("状态列按三态渲染，未完成用低对比灰色而不是失败色", () => {
    render(
      <HealthCheckLogsDesktopTable
        logs={[createHealthCheckLog(1, "success"), createHealthCheckLog(2, "error"), createHealthCheckLog(3, "not_measured")]}
        onOpenDetail={vi.fn()}
      />,
    );

    expect(screen.getByText("成功")).toHaveClass("text-success");
    expect(screen.getByText("失败")).toHaveClass("text-destructive");
    expect(screen.getByText("未完成")).toHaveClass("text-muted-foreground");
  });
});
