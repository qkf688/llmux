import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";

import { testModelProvider } from "@/lib/api";
import { UnauthorizedError } from "@/lib/api/core/client";
import type { Updater } from "@/stores/core/updater";
import type { AssociationBatchTestResult, BatchTestProgress } from "../types";

import { useModelProvidersBatch } from "./use-model-providers-batch";

// 部分 mock：仅替换被测项用到的测试 api，其余导出（含错误形状）保留真实实现。
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  testModelProvider: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const EMPTY_PROGRESS: BatchTestProgress = {
  total: 0,
  completed: 0,
  success: 0,
  failed: 0,
  cancelled: 0,
  testing: 0,
};

/** 有状态替身：vi.fn() 只留调用记录，读不出进度终态与单项结果。 */
function createSetter<T>(initial: T) {
  let state = initial;
  const set = (value: Updater<T>) => {
    state = typeof value === "function" ? (value as (prev: T) => T)(state) : value;
  };
  return { set, get: () => state };
}

function renderBatch(ids: number[]) {
  const results = createSetter<Record<number, AssociationBatchTestResult>>({});
  const progress = createSetter<BatchTestProgress>({ ...EMPTY_PROGRESS });
  const hook = renderHook(() =>
    useModelProvidersBatch({
      selectedModelId: 1,
      fetchModelProviders: vi.fn(),
      filteredModelProviders: [],
      selectedAssociationIds: ids,
      setSelectedAssociationIds: vi.fn(),
      setModelProviders: vi.fn(),
      setBatchDeleteDialogOpen: vi.fn(),
      setBatchDeleting: vi.fn(),
      setBatchUpdatingStatus: vi.fn(),
      setBatchCapabilitiesDialogOpen: vi.fn(),
      setBatchUpdatingCapabilities: vi.fn(),
      setBatchTesting: vi.fn(),
      setBatchTestProgress: progress.set,
      associationTestResults: {},
      setAssociationTestResults: results.set,
    }),
  );
  return { hook, results, progress };
}

async function runBatch(ids: number[]) {
  const { hook, results, progress } = renderBatch(ids);
  await act(async () => {
    await hook.result.current.handleBatchTestSelected();
  });
  return { results, progress };
}

describe("批量测试的登录过期收场", () => {
  beforeEach(() => {
    vi.mocked(toast.success).mockClear();
  });

  it("撞登录过期：计已取消、不计失败", async () => {
    // 错误形状取传输层实际抛出的东西，不是后端文案。
    vi.mocked(testModelProvider).mockRejectedValue(new UnauthorizedError());

    const { progress } = await runBatch([1, 2, 3]);

    expect(progress.get().failed).toBe(0);
    expect(progress.get().cancelled).toBe(3);
    expect(progress.get().completed).toBe(3);
  });

  it("撞登录过期：单项格标已取消、不写失败结论", async () => {
    vi.mocked(testModelProvider).mockRejectedValue(new UnauthorizedError());

    const { results } = await runBatch([7]);

    expect(results.get()[7]).toEqual({ loading: false, success: null, cancelled: true });
  });

  it("汇总提示要带上已取消数（否则只剩「失败 0 个」，等于没交代去向）", async () => {
    vi.mocked(testModelProvider).mockRejectedValue(new UnauthorizedError());

    await runBatch([1, 2]);

    const text = vi.mocked(toast.success).mock.calls[0]?.[0] ?? "";
    expect(text).toContain("已取消 2");
  });

  it("真失败仍计失败（别把 401 的收场扩大成吞掉所有错误）", async () => {
    vi.mocked(testModelProvider).mockRejectedValue(new Error("upstream: 500"));

    const { results, progress } = await runBatch([1]);

    expect(progress.get().failed).toBe(1);
    expect(progress.get().cancelled).toBe(0);
    expect(results.get()[1]).toEqual({
      loading: false,
      success: false,
      error: "upstream: 500",
    });
  });
});