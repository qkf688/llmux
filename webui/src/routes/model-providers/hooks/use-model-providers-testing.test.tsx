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
import { UnauthorizedError } from "@/lib/api/core/client";
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
// 重连重试、last-event-id。abort 已模拟（见 mockImplementation 的 race 分支：真实库
// abort 是 dispose+resolve，不进 onerror/catch 也不进 onclose）；「流停在半空等取消」
// 的长流夹具在各取消用例里以同一语义现场建模。
vi.mock("@microsoft/fetch-event-source", () => ({
  fetchEventSource: vi.fn(),
}));

const setTestMode = vi.fn();

/** 真实库 abort 语义的中止腿：signal 中止即 resolve（dispose+resolve，无任何回调）。 */
function abortResolves(signal: AbortSignal | null | undefined): Promise<void> {
  return new Promise<void>((resolve) => {
    if (!signal) return; // 无 signal = 永不中止，这条腿不结算
    if (signal.aborted) {
      resolve();
      return;
    }
    signal.addEventListener("abort", () => resolve());
  });
}

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

/** ReAct 流专用渲染：流式面板状态走有状态替身，才能读出终态断言。 */
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

  it("能力测试撞登录过期不写「测试失败」：格子回到「没测过」的样子", async () => {
    // mock 错误形状取传输层实际抛出的东西（handleUnauthorized → UnauthorizedError），不是后端文案。
    vi.mocked(testModelProviderVision).mockRejectedValue(new UnauthorizedError());

    const { result } = renderTesting("vision");
    await act(async () => {
      result.current.executeTestNow();
    });

    // 登录过期不是测试失败：不写结论、不标「已取消」，与 SSE 通道同口径（不以「测试失败」掩盖掉线）。
    expect(result.current.visionTestResults[capabilityTestKey(7, "non_stream")]).toEqual({
      loading: false,
      result: null,
    });
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
      const run = (async () => {
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
      })();
      // race 并进 abort 腿：中止以正常返回收场（dispose+resolve，不进 onerror/onclose）。
      run.catch(() => {}); // race 定局后 run 的迟到拒绝不落成未处理拒绝
      await Promise.race([run, abortResolves(options.signal)]);
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

describe("ReAct 流取消收场", () => {
  beforeEach(() => {
    authStore.setState({ token: "test-jwt", user: { id: 1, username: "admin", role: "admin" } });
    vi.mocked(fetchEventSource).mockReset();
  });

  afterEach(() => {
    authStore.setState({ token: null, user: null });
    vi.mocked(fetchEventSource).mockReset();
  });

  it("取消 ReAct 流 → 面板标「已取消」且 loading 复位，半截日志保留", async () => {
    // 长流夹具：吐一段半截内容后停在半空等取消；取消按真实库 abort 语义收场
    // （dispose+resolve，不进 onerror 也不进 onclose）。
    vi.mocked(fetchEventSource).mockImplementation(async (_input, options) => {
      options.onmessage?.({ id: "", event: "message", data: "半截回答" });
      await abortResolves(options.signal);
    });

    const { setReactTestResult, getState } = createReactTestResultSetter();
    const { result } = renderReactTest(setReactTestResult);

    act(() => {
      result.current.executeTestNow();
    });
    expect(getState().loading).toBe(true);
    expect(getState().messages).toContain("半截回答");

    await act(async () => {
      result.current.dialogClose();
    });

    const state = getState();
    expect(state.loading).toBe(false);
    expect(state.cancelled).toBe(true);
    // 取消不是失败：半截日志保留，不写成功/失败结论。
    expect(state.messages).toContain("半截回答");
    expect(state.success).toBeNull();
    expect(state.error).toBeNull();
  });

  it("取消撞上已出结论：保留结论只停转圈，不抹成「已取消」", async () => {
    vi.mocked(fetchEventSource).mockImplementation(async (_input, options) => {
      options.onmessage?.({ id: "", event: "success", data: "测试完成" });
      await abortResolves(options.signal);
    });

    const { setReactTestResult, getState } = createReactTestResultSetter();
    const { result } = renderReactTest(setReactTestResult);

    act(() => {
      result.current.executeTestNow();
    });
    await act(async () => {
      result.current.dialogClose();
    });

    const state = getState();
    expect(state.success).toBe(true);
    expect(state.cancelled).toBe(false);
    expect(state.loading).toBe(false);
  });
});
