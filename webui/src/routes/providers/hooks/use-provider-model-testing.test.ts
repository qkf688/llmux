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

function renderTestingHook(overrides?: {
  allModelsTestResults?: Record<string, ModelTestResult>;
  upstreamTestResults?: Record<string, ModelTestResult>;
  setSelectedAllModels?: Mock;
  setSelectedUpstreamModels?: Mock;
}) {
  const setUpstreamTestResults = vi.fn();
  const setAllModelsTestResults = vi.fn();
  const setSelectedAllModels = overrides?.setSelectedAllModels ?? vi.fn();
  const setSelectedUpstreamModels = overrides?.setSelectedUpstreamModels ?? vi.fn();

  const hook = renderHook(() =>
    useProviderModelTesting({
      allModelsProvider: { ID: 7, Name: "p", Type: "openai", Config: "{}", Console: "", Proxy: "" },
      modelsOpenId: 7,
      filteredAllModels: ["m1"],
      filteredProviderModels: [{ id: "m1", object: "model", created: 0, owned_by: "o" }],
      selectedAllModels: [],
      selectedUpstreamModels: [],
      allModelsTestResults: overrides?.allModelsTestResults ?? {},
      upstreamTestResults: overrides?.upstreamTestResults ?? {},
      setSelectedAllModels,
      setSelectedUpstreamModels,
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
  const lastEntry = (setter: Mock, key = "m1::0"): ModelTestResult => {
    const calls = setter.mock.calls;
    const updater = calls[calls.length - 1][0] as ResultsUpdater;
    return updater({})[key];
  };

  return { hook, lastEntry, setUpstreamTestResults, setAllModelsTestResults, setSelectedAllModels, setSelectedUpstreamModels };
}

describe("useProviderModelTesting 测试结果判定", () => {
  beforeEach(() => {
    testProviderModelMock.mockReset();
  });

  it("passed:true → success 为 true，结果键为 m1::0", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 3, group_name: "A 组" });

    const ok = await hook.result.current.handleTestUpstreamModel("m1");

    expect(ok).toBe("success");
    expect(lastEntry(setUpstreamTestResults, "m1::0")).toMatchObject({ loading: false, success: true });
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

    expect(ok).toBe("failed");
    expect(lastEntry(setUpstreamTestResults, "m1::0")).toMatchObject({
      loading: false,
      success: false,
      error: "[认证错误] 提供商返回错误",
    });
  });

  it("passed 缺失（异常响应形状）→ 按失败处理，不乐观判通过", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ group_id: 0 });

    const ok = await hook.result.current.handleTestUpstreamModel("m1");

    expect(ok).toBe("failed");
    expect(lastEntry(setUpstreamTestResults, "m1::0").success).toBe(false);
  });

  it("传输层抛错 → success 为 false 并用错误消息兜底", async () => {
    const { hook, lastEntry, setAllModelsTestResults } = renderTestingHook();
    testProviderModelMock.mockRejectedValue(new Error("API request failed: 500"));

    const ok = await hook.result.current.handleTestAllModel("m1");

    expect(ok).toBe("failed");
    expect(lastEntry(setAllModelsTestResults, "m1::0")).toMatchObject({
      loading: false,
      success: false,
      error: "API request failed: 500",
    });
  });

  it("取消（AbortError）→ 标 cancelled 且不算失败", async () => {
    const { hook, lastEntry, setAllModelsTestResults } = renderTestingHook();
    testProviderModelMock.mockRejectedValue(new DOMException("Aborted", "AbortError"));

    const outcome = await hook.result.current.handleTestAllModel("m1", undefined, new AbortController().signal);

    expect(outcome).toBe("cancelled");
    expect(lastEntry(setAllModelsTestResults, "m1::0")).toMatchObject({
      loading: false,
      success: null,
      cancelled: true,
    });
  });

  it("自动口径不传 group_id（链路由后端选定）", async () => {
    const { hook } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 3 });

    await hook.result.current.handleTestUpstreamModel("m1");

    // groupId 未传时第三参为 undefined，API 层按 falsy 处理为自动口径；第四参是取消信号（单点调用不传）
    expect(testProviderModelMock).toHaveBeenCalledWith(7, "m1", undefined, undefined);
  });

  // AC-1：点分组徽标只测该组
  it("传 groupId 时锁定该分组，结果键为 m1::5", async () => {
    const { hook, lastEntry, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 5, group_name: "B 组" });

    const ok = await hook.result.current.handleTestUpstreamModel("m1", 5);

    expect(ok).toBe("success");
    expect(testProviderModelMock).toHaveBeenCalledWith(7, "m1", 5, undefined);
    expect(lastEntry(setUpstreamTestResults, "m1::5")).toMatchObject({ loading: false, success: true });
    // 锁组测试不写自动口径键
    expect(lastEntry(setUpstreamTestResults, "m1::0")).toBeUndefined();
  });

  // AC-2：自动口径按响应回传分组把状态落到正确徽标
  it("自动口径完成后，结果同时落到 ::0 与回传分组 ::3", async () => {
    const { hook, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 3, group_name: "A 组" });

    await hook.result.current.handleTestUpstreamModel("m1");

    const calls = setUpstreamTestResults.mock.calls;
    const updater = calls[calls.length - 1][0] as (
      prev: Record<string, ModelTestResult>,
    ) => Record<string, ModelTestResult>;
    const result = updater({});
    expect(result["m1::0"]).toMatchObject({ success: true });
    expect(result["m1::3"]).toMatchObject({ success: true });
  });

  it("自动口径 group_id 为 0 时不写分组键", async () => {
    const { hook, setUpstreamTestResults } = renderTestingHook();
    testProviderModelMock.mockResolvedValue({ passed: true, group_id: 0 });

    await hook.result.current.handleTestUpstreamModel("m1");

    const calls = setUpstreamTestResults.mock.calls;
    const updater = calls[calls.length - 1][0] as (
      prev: Record<string, ModelTestResult>,
    ) => Record<string, ModelTestResult>;
    const result = updater({});
    expect(result["m1::0"]).toMatchObject({ success: true });
    expect(result["m1::3"]).toBeUndefined();
  });
});

describe("select 成功/失败按自动口径条目拆键", () => {
  beforeEach(() => {
    testProviderModelMock.mockReset();
  });

  it("selectUpstreamSuccessful 只取 ::0 条目并拆出模型名", () => {
    const setSelectedUpstreamModels = vi.fn();
    const { hook } = renderTestingHook({
      upstreamTestResults: {
        "m1::0": { loading: false, success: true },
        "m1::3": { loading: false, success: false },
        "m2::0": { loading: false, success: false },
      },
      setSelectedUpstreamModels,
    });

    hook.result.current.selectUpstreamSuccessful();

    expect(setSelectedUpstreamModels).toHaveBeenCalledWith(["m1"]);
  });

  it("selectUpstreamFailed 只取 ::0 条目并拆出模型名", () => {
    const setSelectedUpstreamModels = vi.fn();
    const { hook } = renderTestingHook({
      upstreamTestResults: {
        "m1::0": { loading: false, success: true },
        "m2::0": { loading: false, success: false },
        "m3::5": { loading: false, success: false },
      },
      setSelectedUpstreamModels,
    });

    hook.result.current.selectUpstreamFailed();

    expect(setSelectedUpstreamModels).toHaveBeenCalledWith(["m2"]);
  });

  it("clearUpstreamTestResults 清空结果表", () => {
    const { hook, setUpstreamTestResults } = renderTestingHook();

    hook.result.current.clearUpstreamTestResults();

    expect(setUpstreamTestResults).toHaveBeenCalledWith({});
  });
});
