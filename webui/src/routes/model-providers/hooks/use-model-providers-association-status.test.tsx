import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { getModelProviderHealthStatus, getModelProviderStatus, type Model } from "@/lib/api";

import { createMockAssociation } from "../test-fixtures";
import { useModelProvidersAssociationStatus } from "./use-model-providers-association-status";

vi.mock("@/lib/api", () => ({
  getModelProviderStatus: vi.fn(),
  getModelProviderHealthStatus: vi.fn(),
}));

const models = [{ ID: 1, Name: "gpt-test" } as Model];

describe("useModelProvidersAssociationStatus", () => {
  it("健康检测三态原样进 healthStatus（不压成布尔），请求成败仍走布尔", async () => {
    vi.mocked(getModelProviderStatus).mockResolvedValue([true, false]);
    vi.mocked(getModelProviderHealthStatus).mockResolvedValue(["success", "error", "not_measured"]);

    const { result } = renderHook(() => useModelProvidersAssociationStatus(models));

    await act(async () => {
      await result.current.loadProviderStatus([createMockAssociation(10, 20)], 1);
    });

    expect(vi.mocked(getModelProviderHealthStatus)).toHaveBeenCalledWith(10);
    expect(result.current.providerStatus[10]).toEqual([true, false]);
    expect(result.current.healthStatus[10]).toEqual(["success", "error", "not_measured"]);
  });

  it("单条拉取失败时两侧都回落空数组（画成「无数据」而不是复用上一次的三态）", async () => {
    vi.mocked(getModelProviderStatus).mockResolvedValue([true]);
    vi.mocked(getModelProviderHealthStatus).mockResolvedValue(["success"]);
    vi.mocked(getModelProviderHealthStatus).mockRejectedValueOnce(new Error("boom"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    const { result } = renderHook(() => useModelProvidersAssociationStatus(models));

    await act(async () => {
      await result.current.loadProviderStatus([createMockAssociation(11, 20)], 1);
    });

    expect(result.current.healthStatus[11]).toEqual([]);
    consoleError.mockRestore();
  });
});
