import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { HealthCheckLog } from "@/lib/api";

import { HealthCheckLogsDesktopTable } from "./health-check-logs-desktop-table";

function createLog(ID: number, status: HealthCheckLog["status"]): HealthCheckLog {
  return {
    ID,
    CreatedAt: "2026-01-01T00:00:00Z",
    model_provider_id: ID,
    model_name: "gpt-test",
    provider_name: "prov",
    provider_model: "gpt-4o",
    status,
    error: "",
    response_time: 12,
    checked_at: "2026-01-01T00:00:00Z",
  };
}

describe("HealthCheckLogsDesktopTable", () => {
  it("状态列按三态渲染，未完成用低对比灰色而不是失败色", () => {
    render(
      <HealthCheckLogsDesktopTable
        logs={[createLog(1, "success"), createLog(2, "error"), createLog(3, "not_measured")]}
        onOpenDetail={vi.fn()}
      />,
    );

    expect(screen.getByText("成功")).toHaveClass("text-success");
    expect(screen.getByText("失败")).toHaveClass("text-destructive");
    expect(screen.getByText("未完成")).toHaveClass("text-muted-foreground");
  });
});
