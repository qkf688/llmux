import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";

// testProviderModel 需要按用例控制返回形状（#19 起失败也回 HTTP 200 + passed:false），
// 引用必须在 vi.mock 工厂之外持有。
const { testProviderModelMock } = vi.hoisted(() => ({ testProviderModelMock: vi.fn() }));

vi.mock("@/lib/api", () => ({
  testProviderModel: testProviderModelMock,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

import type { ModelTestResult } from "../types";
import { useProviderModelTesting } from "./use-provider-model-testing";

function renderTestingHook() {
  const setUpstreamTestResults = vi.fn();
  const setAllModelsTestResults = vi.fn();

  const hook = renderHook(() =>
    useProviderModelTesting({
      allModelsProvider: { ID: 7, Name: "p", Type: "openai", Config: "{}", Console: "", Proxy: "" },
      modelsOpenId: 7,
      filteredAllModels: ["m1"],
      filteredProviderModels: [{ id: "m1", object: "model", created: 0, owned_by: "o" }],
      selectedAllModels: [],
      selectedUpstreamModels: [],
      allModelsTestResults: {},
      upstreamTestResults: {},
      setSelectedAllModels: vi.fn(),
      setSelectedUpstreamModels: vi.fn(),
      setAllModelsTestResults,
      setUpstreamTestResults,
      setBatchTesting: vi.fn(),
      setBatchTestProgress: vi.fn(),
      setUpstreamBatchTesting: vi.fn(),
      setUpstreamBatchTestProgress: vi.fn(),
    })
  );

  // hook 用函数式更新写入结果：取最后一次 updater 在空表上求值，即得该模型的落点。
  type ResultsUpdater = (prev: Record<string, ModelTestResult>) => Record<string, ModelTestResult>;
  const lastEntry = (setter: Mock): ModelTestResult => {
    const calls = setter.mock.calls;
    const updater = calls[calls.length - 1][0] as ResultsUpdater;
    return updater({}).m1;
  };

  return { hook, lastEntry, setUpstreamTestResults, setAllModelsTestResults };
}

describe("useProviderModelTesting 测试结果判定", () => {
  beforeEach(() => {
    testProviderModelMock.mockReset();
  });

  it("passed:true → success 为 true", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 3, group_name: "A 组" });

    const ok = await hook.result.current.handleTestUpstreamModel("m1");

    expect(ok).toBe(true);
    expect(lastEntry(setUpstreamTestResults)).toMatchObject({ loading: false, success: true });
  });

  // #19 回归：后端把「测试失败」改成 code 200 + passed:false（失败也要回传分组供归位）。
  // 前端若仍只靠抛错判定，这条会把它显示成通过——所有 ✕ 静默变 ✓。
  it("passed:false → success 为 false 且带上后端 error 文本", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({
      passed: false,
      error: "[认证错误] 提供商返回错误",
      error_type: "auth",
      group_id: 3,
      group_name: "A 组",
    });

    const ok = await hook.result.current.handleTestUpstreamModel("m1");

    expect(ok).toBe(false);
    expect(lastEntry(setUpstreamTestResults)).toMatchObject({
      loading: false,
      success: false,
      error: "[认证错误] 提供商返回错误",
    });
  });

  it("passed 缺失（异常响应形状）→ 按失败处理，不乐观判通过", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ group_id: 0 });

    const ok = await hook.result.current.handleTestUpstreamModel("m1");

    expect(ok).toBe(false);
    expect(lastEntry(setUpstreamTestResults).success).toBe(false);
  });

  it("传输层抛错 → success 为 false 并用错误消息兜底", async () => {
    const { hook, lastEntry, setAllModelsTestResults } = renderTestingHook();
    testProviderModelMock.mockRejectedValue(new Error("API request failed: 500"));

    const ok = await hook.result.current.handleTestAllModel("m1");

    expect(ok).toBe(false);
    expect(lastEntry(setAllModelsTestResults)).toMatchObject({
      loading: false,
      success: false,
      error: "API request failed: 500",
    });
  });

  it("自动口径不传 group_id（链路由后端选定）", async () => {
    const { hook } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 3 });

    await hook.result.current.handleTestUpstreamModel("m1");

    expect(testProviderModelMock).toHaveBeenCalledWith(7, "m1");
  });
});
