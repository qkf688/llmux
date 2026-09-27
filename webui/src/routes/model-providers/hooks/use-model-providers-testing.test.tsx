import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fetchEventSource } from "@microsoft/fetch-event-source";

import {
  testModelProvider,
  testModelProviderStructuredOutput,
  testModelProviderThinking,
  testModelProviderVision,
  type ModelProviderTestResult,
} from "@/lib/api";
import { authStore } from "@/stores/auth";
import { consumeLoginExpiryNotice, hasLoginExpiryNotice } from "@/stores/auth/login-expiry-notice";
import type { Updater } from "@/stores/core/updater";
import type { ReactTestResultState } from "@/stores/model-providers/types";

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

// fetchEventSource 是被测 hook 的流传输边界（自持长连接，不经信封通道）。
// mock 不是随手桩：实现复刻真实库（lib/esm/fetch.js）的关键语义——默认 onopen 校验
// 响应 content-type 必须是 event-stream；错误进 onerror，而 onerror 的契约是双向的：
// **抛出才终止**整条连接（Promise reject），正常返回则把返回值当重试间隔**重连**。
// 故 mock 把「onerror 未抛出」判为终止契约被破坏直接失败——否则删掉调用点的终止
// throw 用例仍全绿，锁不住「必须终止」这半边（变异检验实锤过：真实库里那等于
// 拿同一过期 token 每秒重连死循环）。刻意未模拟的语义（写在这里防误导后来者）：
// 重连重试、abort（真实库 abort 是 dispose+resolve，不进 onerror/catch）、last-event-id。
vi.mock("@microsoft/fetch-event-source", () => ({
  fetchEventSource: vi.fn(),
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

    expect(testModelProviderVision).toHaveBeenCalledWith(7, "non_stream", expect.any(AbortSignal));
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

    expect(testModelProviderThinking).toHaveBeenCalledWith(7, "stream", expect.any(AbortSignal));
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

  it("关弹窗中止在飞测试：loading 复位、标「已取消」，不写成失败", async () => {
    // 模拟真实取消：api 在 signal 中止时抛 AbortError（fetch 的实际行为）。
    vi.mocked(testModelProviderVision).mockImplementation(
      (_id, _mode, signal) =>
        new Promise<ModelProviderTestResult>((_resolve, reject) => {
          signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
        }),
    );

    const { result } = renderTesting("vision");
    act(() => {
      result.current.executeTestNow();
    });
    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]?.loading).toBe(true);

    await act(async () => {
      result.current.dialogClose();
    });

    const entry = result.current.visionTestResults[capabilityTestKey(7, "non_stream")];
    expect(entry.loading).toBe(false);
    expect(entry.cancelled).toBe(true);
    // 已取消不是失败：不产出结论，也不带失败文案。
    expect(entry.result).toBeNull();
  });

  it("同格重复测试：旧请求的迟到结果不得覆写新结果", async () => {
    let resolveOld: ((value: ModelProviderTestResult) => void) | undefined;
    vi.mocked(testModelProviderVision)
      .mockImplementationOnce(
        () =>
          new Promise<ModelProviderTestResult>((resolve) => {
            resolveOld = resolve;
          }),
      )
      .mockResolvedValueOnce({ passed: true, vision_text: "新结果" });

    const { result } = renderTesting("vision");
    act(() => {
      result.current.executeTestNow();
    });
    // 第二次打同一格：按「同格取消」口径应先中止上一次
    await act(async () => {
      result.current.executeTestNow();
    });
    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]?.result).toEqual({
      passed: true,
      vision_text: "新结果",
    });

    // 旧请求迟到才 resolve：它已不代表该格，不得覆写新结论
    await act(async () => {
      resolveOld?.({ passed: false, vision_text: "旧结果" });
    });
    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]?.result).toEqual({
      passed: true,
      vision_text: "新结果",
    });
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

describe("SSE 流 401 处置", () => {
  const fetchMock = vi.fn();
  let locationAssignments: string[];

  /** 401 拒绝的通行形状：JSON 错误页而非 event-stream（默认 onopen 会因此抛错）。 */
  function unauthorizedResponse() {
    return {
      ok: false,
      status: 401,
      headers: new Headers({ "Content-Type": "application/json" }),
    } as unknown as Response;
  }

  /** 真实库默认 onopen：content-type 必须是 event-stream，否则抛错进 onerror。 */
  const defaultOnOpen = async (response: Response) => {
    const contentType = response.headers.get("content-type");
    if (!contentType || !contentType.startsWith("text/event-stream")) {
      throw new Error(`Expected content-type to be text/event-stream, Actual: ${contentType}`);
    }
  };

  /** setReactTestResult 的有状态替身：vi.fn() 只留调用记录，读不出面板终态。 */
  function createReactTestResultSetter() {
    let state: ReactTestResultState = { loading: false, messages: "", success: null, error: null };
    return {
      setReactTestResult: (next: Updater<ReactTestResultState>) => {
        state = typeof next === "function" ? next(state) : next;
      },
      getState: () => state,
    };
  }

  function renderReactTest(setReactTestResult: (next: Updater<ReactTestResultState>) => void) {
    return renderHook(() =>
      useModelProvidersTesting({
        setTestDialogOpen: vi.fn(),
        setSelectedTestId: vi.fn(),
        setTestType: vi.fn(),
        setTestMode: vi.fn(),
        setReactTestResult,
        selectedTestId: 7,
        testType: "react",
        testMode: "non_stream",
      }),
    );
  }

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
    authStore.setState({ token: "test-jwt", user: { id: 1, username: "admin", role: "admin" } });
    // 一次性过期提示是「读后即焚」的会话级标记：不清会跨用例残留，
    // 让「已设标记不再跳」的去重判据误判成上个用例写过的旧标记。
    sessionStorage.clear();
    consumeLoginExpiryNotice();

    // jsdom 的 location.href 赋值只告警不可断言：换成可写对象捕获跳转（同 client.test.ts）。
    locationAssignments = [];
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        get href() {
          return locationAssignments.at(-1) ?? "";
        },
        set href(value: string) {
          locationAssignments.push(value);
        },
      },
    });

    vi.mocked(fetchEventSource).mockImplementation(async (input, options) => {
      const doFetch = options.fetch ?? globalThis.fetch;
      try {
        const response = await doFetch(input, {
          method: options.method,
          headers: options.headers,
          signal: options.signal,
        });
        await (options.onopen ?? defaultOnOpen)(response);
      } catch (err) {
        // 真实库：onerror 抛出才终止整条连接（reject）；正常返回即重连。mock 不模拟
        // 重连，把「未抛出」直接判失败——终止契约的另一半必须有测试锁（见顶部注释）。
        options.onerror?.(err);
        throw new Error("mock: onerror 未抛出终止——真实库会以返回值为间隔重连，终止契约被破坏");
      }
      options.onclose?.();
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    authStore.setState({ token: null, user: null });
    vi.mocked(fetchEventSource).mockReset();
  });

  it("SSE 流中途 401：走统一失效处置（清会话、跳登录页、写过期提示标记）", async () => {
    fetchMock.mockResolvedValue(unauthorizedResponse());
    const { setReactTestResult } = createReactTestResultSetter();
    const { result } = renderReactTest(setReactTestResult);

    await act(async () => {
      result.current.executeTestNow();
    });

    expect(authStore.getState().token).toBeNull();
    expect(locationAssignments).toEqual(["/login"]);
    expect(hasLoginExpiryNotice()).toBe(true);
  });

  it("SSE 流 401 不以「测试失败」掩盖掉线：面板不写失败结论", async () => {
    fetchMock.mockResolvedValue(unauthorizedResponse());
    const { setReactTestResult, getState } = createReactTestResultSetter();
    const { result } = renderReactTest(setReactTestResult);

    await act(async () => {
      result.current.executeTestNow();
    });

    const state = getState();
    expect(state.error).toBeNull();
    expect(state.success).toBeNull();
    expect(state.loading).toBe(false);
  });
});
