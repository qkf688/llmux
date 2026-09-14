import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  testModelProvider,
  testModelProviderStructuredOutput,
  testModelProviderThinking,
  testModelProviderVision,
} from "@/lib/api";

import type { TestMode, TestType } from "../types";
import { capabilityTestKey, useModelProvidersTesting } from "./use-model-providers-testing";

// 部分 mock：仅替换四个测试 api，保留 @/lib/api 其余导出（错误形状等被其他模块共享）。
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  testModelProvider: vi.fn(),
  testModelProviderStructuredOutput: vi.fn(),
  testModelProviderThinking: vi.fn(),
  testModelProviderVision: vi.fn(),
}));

const setTestMode = vi.fn();

function renderTesting(testType: TestType, testMode: TestMode = "non_stream") {
  return renderHook(() =>
    useModelProvidersTesting({
      setTestDialogOpen: vi.fn(),
      setSelectedTestId: vi.fn(),
      setTestType: vi.fn(),
      setTestMode,
      setReactTestResult: vi.fn(),
      selectedTestId: 7,
      testType,
      testMode,
    }),
  );
}

describe("useModelProvidersTesting", () => {
  beforeEach(() => {
    vi.mocked(testModelProvider).mockReset();
    vi.mocked(testModelProviderStructuredOutput).mockReset();
    vi.mocked(testModelProviderThinking).mockReset();
    vi.mocked(testModelProviderVision).mockReset();
    setTestMode.mockReset();
  });

  it("按 testType 分发表派发执行：vision 只打 vision api，不落入其他类型分支", async () => {
    vi.mocked(testModelProviderVision).mockResolvedValue({ passed: true, vision_text: "The image is red." });

    const { result } = renderTesting("vision");
    await act(async () => {
      result.current.executeTestNow();
    });

    expect(testModelProviderVision).toHaveBeenCalledWith(7, "non_stream");
    // 回归锁定：旧 if/else 链会把 vision 静默落进 structured_output 的 else 分支
    expect(testModelProviderStructuredOutput).not.toHaveBeenCalled();
    expect(testModelProviderThinking).not.toHaveBeenCalled();
    expect(testModelProvider).not.toHaveBeenCalled();
    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]).toEqual({
      loading: false,
      result: { passed: true, vision_text: "The image is red." },
    });
  });

  it("各类型执行单元状态隔离：一种测试的结果不写入另一种的 map", async () => {
    vi.mocked(testModelProviderThinking).mockResolvedValue({ passed: true, thinking_text: "2^10=1024" });

    const { result } = renderTesting("thinking");
    await act(async () => {
      result.current.executeTestNow();
    });

    expect(result.current.thinkingTestResults[capabilityTestKey(7, "non_stream")]?.result?.passed).toBe(true);
    expect(result.current.visionTestResults).toEqual({});
    expect(result.current.structuredTestResults).toEqual({});
  });

  it("mode 透传到 api：流式模式打的是同一端点 + stream 参数", async () => {
    vi.mocked(testModelProviderThinking).mockResolvedValue({ passed: true, mode: "stream" });

    const { result } = renderTesting("thinking", "stream");
    await act(async () => {
      result.current.executeTestNow();
    });

    expect(testModelProviderThinking).toHaveBeenCalledWith(7, "stream");
  });

  it("同一关联的两种模式各占一格：切模式不覆盖上一模式的结论", async () => {
    vi.mocked(testModelProviderThinking)
      .mockResolvedValueOnce({ passed: true, mode: "non_stream" })
      .mockResolvedValueOnce({ passed: false, mode: "stream", error: "上游不支持流式" });

    const nonStream = renderTesting("thinking", "non_stream");
    await act(async () => {
      nonStream.result.current.executeTestNow();
    });
    const stream = renderTesting("thinking", "stream");
    await act(async () => {
      stream.result.current.executeTestNow();
    });

    // 「非流式通过、流式失败」必须同时可见——这正是流式模式要检出的缺陷形状。
    expect(nonStream.result.current.thinkingTestResults[capabilityTestKey(7, "non_stream")]?.result?.passed).toBe(true);
    expect(stream.result.current.thinkingTestResults[capabilityTestKey(7, "stream")]?.result?.passed).toBe(false);
    // 各自只看得见自己那一格（切换模式不会读到旧结论）。
    expect(nonStream.result.current.thinkingTestResults[capabilityTestKey(7, "stream")]).toBeUndefined();
    expect(stream.result.current.thinkingTestResults[capabilityTestKey(7, "non_stream")]).toBeUndefined();
  });

  it("api 抛错时执行单元兜底为 {passed:false, error}，loading 复位", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(testModelProviderVision).mockRejectedValue(new Error("boom"));

    const { result } = renderTesting("vision");
    await act(async () => {
      result.current.executeTestNow();
    });

    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]).toEqual({
      loading: false,
      result: { passed: false, error: "测试失败: boom" },
    });
    consoleError.mockRestore();
  });

  it("未选中关联（selectedTestId=null）时执行不发起任何请求", async () => {
    const { result } = renderHook(() =>
      useModelProvidersTesting({
        setTestDialogOpen: vi.fn(),
        setSelectedTestId: vi.fn(),
        setTestType: vi.fn(),
        setTestMode,
        setReactTestResult: vi.fn(),
        selectedTestId: null,
        testType: "vision",
        testMode: "stream",
      }),
    );
    await act(async () => {
      result.current.executeTestNow();
    });

    expect(testModelProviderVision).not.toHaveBeenCalled();
  });

  it("打开弹窗回到非流式默认模式", () => {
    const { result } = renderTesting("vision", "stream");
    act(() => {
      result.current.handleTest(9);
    });

    expect(setTestMode).toHaveBeenCalledWith("non_stream");
  });
});
