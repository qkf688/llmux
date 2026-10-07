import { useCallback, useEffect, useRef, useState } from "react";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { isAbortError, toErrorMessage } from "@/lib/errors";
import {
  testModelProvider,
  testModelProviderStructuredOutput,
  testModelProviderThinking,
  testModelProviderVision,
  type ModelProviderTestResult,
} from "@/lib/api";
import { requireAuthTokenOrRedirect, fetchHandlingUnauthorized, UnauthorizedError } from "@/lib/api/core/client";
import type { Setter } from "@/stores/core/updater";
import type { ReactTestResultState } from "@/stores/model-providers/types";
import type { CapabilityTestKey, TestMode, TestType } from "../types";

/**
 * ReAct 流失败的兜底文案：只有取不到任何具体原因时才用。
 * 两个收场分支（`onerror` 与 `catch`）共用同一取法，禁止任一处再写死「测试失败」——
 * 那会把已写进面板的具体原因（超时/断连/上游报错）抹成一句无区分度的话，
 * 排查时只能翻 console。
 */
const REACT_TEST_FALLBACK_ERROR = "测试过程中发生错误";

// 结果格三态（进行中/通过/失败）之外多一个「已取消」：它既不是失败（不证明供应商不行）
// 也不是未测（用户明确停过），所以在 entry 上单独标记，不复用空 result 表达。
type CapabilityTestEntry = {
  loading: boolean;
  result: ModelProviderTestResult | null;
  cancelled?: boolean;
};

export type CapabilityTestResults = Record<CapabilityTestKey, CapabilityTestEntry>;

type CapabilityTestApi = (id: number, mode: TestMode, signal?: AbortSignal) => Promise<ModelProviderTestResult>;

type UseModelProvidersTestingInput = {
  setTestDialogOpen: (open: boolean) => void;
  setSelectedTestId: (id: number) => void;
  setTestType: (value: TestType) => void;
  setTestMode: (value: TestMode) => void;
  setReactTestResult: Setter<ReactTestResultState>;

  selectedTestId: number | null;
  testType: TestType;
  testMode: TestMode;
};

// 结果存储键：关联 id + 模式。同一个关联的流式与非流式结论都要看得见
// （「非流式通过、流式失败」正是本次要检出的缺陷），共用一格会让切换模式后读到旧结论。
export function capabilityTestKey(id: number, mode: TestMode): CapabilityTestKey {
  return `${id}:${mode}`;
}

// JSON 型能力测试执行单元：按「关联 + 模式」记 loading/结果 + 取消与异常兜底。
// 此前 connectivity/structured/thinking/vision 各抄一份同款 try/catch（第 5 类测试就要抄第 5 份），
// 收敛后新增测试类型只需多调一次本 hook（OCP）。api 传模块级函数引用，天然稳定。
function useCapabilityTest(api: CapabilityTestApi) {
  const [results, setResults] = useState<CapabilityTestResults>({});
  // 按格持有在飞请求：同一格重复测试要先停掉上一次（否则旧结果会覆写新结果，
  // 且流式那条上游会话仍跑满计费）。abortAll 供关弹窗/切页面一次清干净。
  const controllersRef = useRef<Map<string, AbortController>>(new Map());

  const run = useCallback(
    async (id: number, mode: TestMode): Promise<ModelProviderTestResult | null> => {
      const key = capabilityTestKey(id, mode);
      controllersRef.current.get(key)?.abort();
      const controller = new AbortController();
      controllersRef.current.set(key, controller);
      // 只有仍代表该格「当前请求」的那一次才允许写结果：被取消后该格可能已被新请求接管，
      // 旧请求的迟到 resolve/异常不得覆写新结果。
      const isCurrent = () => controllersRef.current.get(key) === controller;

      try {
        setResults((prev) => ({ ...prev, [key]: { loading: true, result: null } }));
        const result = await api(id, mode, controller.signal);
        if (isCurrent()) {
          setResults((prev) => ({ ...prev, [key]: { loading: false, result } }));
        }
        return result;
      } catch (err) {
        if (isAbortError(err)) {
          if (isCurrent()) {
            setResults((prev) => ({ ...prev, [key]: { loading: false, result: null, cancelled: true } }));
          }
          return null;
        }
        // 登录过期不是「测试失败」：失效出口已处置（标记/清会话/跳转），这里按非失败终止
        // 收场——复位 loading、不写结论、不标「已取消」，格子回到「没测过」的样子，
        // 与 SSE 通道同口径，不以「测试失败」掩盖掉线。
        if (err instanceof UnauthorizedError) {
          if (isCurrent()) {
            setResults((prev) => ({ ...prev, [key]: { loading: false, result: null } }));
          }
          return null;
        }
        const message = `测试失败: ${toErrorMessage(err)}`;
        if (isCurrent()) {
          setResults((prev) => ({
            ...prev,
            [key]: { loading: false, result: { passed: false, error: message } },
          }));
        }
        console.error(err);
        return { passed: false, error: message };
      } finally {
        if (isCurrent()) {
          controllersRef.current.delete(key);
        }
      }
    },
    [api]
  );

  const abortAll = useCallback(() => {
    // 不 clear：各请求在 catch/finally 里自行判断「是否仍是该格当前请求」再清理。
    // 提前 clear 会让它们判定为非当前请求，连「已取消」状态都不写了。
    for (const controller of controllersRef.current.values()) {
      controller.abort();
    }
  }, []);

  return { results, run, abortAll };
}

export function useModelProvidersTesting({
  setTestDialogOpen,
  setSelectedTestId,
  setTestType,
  setTestMode,
  setReactTestResult,
  selectedTestId,
  testType,
  testMode,
}: UseModelProvidersTestingInput) {
  const connectivityTest = useCapabilityTest(testModelProvider);
  const structuredTest = useCapabilityTest(testModelProviderStructuredOutput);
  const thinkingTest = useCapabilityTest(testModelProviderThinking);
  const visionTest = useCapabilityTest(testModelProviderVision);

  // abortAll 是 useCallback([]) 的稳定引用，解构出来做依赖，避免因 hook 返回对象每次新建而重建回调。
  const { abortAll: abortConnectivity } = connectivityTest;
  const { abortAll: abortStructured } = structuredTest;
  const { abortAll: abortThinking } = thinkingTest;
  const { abortAll: abortVision } = visionTest;

  const currentControllerRef = useRef<AbortController | null>(null);

  // ReAct 走 fetchEventSource 自持一条流，与四类能力测试分开持有控制器（它是单一在飞，不按格）。
  const abortAllCapabilityTests = useCallback(() => {
    abortConnectivity();
    abortStructured();
    abortThinking();
    abortVision();
  }, [abortConnectivity, abortStructured, abortThinking, abortVision]);

  const abortAllTests = useCallback(() => {
    abortAllCapabilityTests();
    // ReAct 这条不置空 ref：置空会让「用户取消」与「被新测试顶掉」同形——取消收场会被
    // 当成旧流的迟到写入拦掉（面板 loading 残留、「已取消」恒不置位）。用户取消要写收场，
    // ref 保留给该流 await 后的收场与 finally 自解绑；只有「新测试接管」（handleTest /
    // handleReactTest 换掉 ref）才拦旧流写入。
    currentControllerRef.current?.abort();
  }, [abortAllCapabilityTests]);

  // 切走页面（组件卸载）时同样要停：否则能力测试/ReAct 会在后台继续跑满并计费。
  useEffect(() => () => abortAllTests(), [abortAllTests]);

  const dialogClose = useCallback(() => {
    // 关弹窗 = 用户不想看了：停掉所有在飞的测试，上游会话随之取消、不再计费。
    abortAllTests();
    setTestDialogOpen(false);
  }, [abortAllTests, setTestDialogOpen]);

  const handleTest = useCallback(
    (id: number) => {
      // 打开新弹窗即停掉上一次的 ReAct 流；能力测试按格管理，不需在这里清。
      currentControllerRef.current?.abort();
      currentControllerRef.current = null;
      setSelectedTestId(id);
      setTestType("connectivity");
      // 打开弹窗回到默认模式：若沿用上一轮的「流式」，用户会以为点击立即出结果，
      // 实际要多等十几秒到几十秒（流式要读完整条 SSE）。
      setTestMode("non_stream");
      setTestDialogOpen(true);
      setReactTestResult({
        loading: false,
        messages: "",
        success: null,
        error: null,
      });
    },
    [setReactTestResult, setSelectedTestId, setTestDialogOpen, setTestMode, setTestType]
  );

  const handleReactTest = useCallback(
    async (id: number) => {
      // 同一时刻只允许一条 ReAct 流：新的先停掉旧的（否则旧流的事件会追加进新结果）。
      currentControllerRef.current?.abort();

      // 复跑要清空上一轮结论（success/error 一并）：否则取消收场把旧结论误当「本流已出
      // 结论」，会保留上一轮结论、漏标「已取消」（与 useCapabilityTest 开跑清 result 对称）。
      setReactTestResult((prev) => ({
        ...prev,
        messages: "",
        loading: true,
        cancelled: false,
        success: null,
        error: null,
      }));

      const controller = new AbortController();
      currentControllerRef.current = controller;

      // 只有仍代表「当前这条流」的事件才允许写结果：关弹窗/卸载/新流接管之后，
      // 旧流的迟到事件（含 abort 触发的那次 onerror）不得再改状态。
      const applyIfCurrent = (updater: (prev: ReactTestResultState) => ReactTestResultState) => {
        if (currentControllerRef.current !== controller) return;
        setReactTestResult(updater);
      };

      // 取消收场的单一口径（onerror 防御分支与 await 后主路径共用）：取消不抹已出结论——
      // 本流 success/error 已写上就保留结论只停转圈，否则标「已取消」；半截日志保留。
      const settleCancelled = () =>
        applyIfCurrent((prev) =>
          prev.success !== null ? { ...prev, loading: false } : { ...prev, loading: false, cancelled: true },
        );

      try {
        // 长连接不经信封通道（fetchEventSource 自持连接），故只借统一入口的鉴权与
        // 凭证失效处置（清会话 + 跳登录页），不自拼 Authorization。无凭证时它直抛
        // UnauthorizedError（走下面 catch 的过期收场）。
        const token = requireAuthTokenOrRedirect();

        await fetchEventSource(`/api/test/react/${id}`, {
          method: "GET",
          headers: { Authorization: `Bearer ${token}` },
          signal: controller.signal,
          // 连接建立时的 401（token 中途失效）经统一失效出口处置（标记/清会话/跳登录页），
          // 与信封通道同一 owner；自拼 fetch 或在回调里另造 401 处置都是旁路。
          fetch: fetchHandlingUnauthorized,
          onmessage(event) {
            applyIfCurrent((prev) => {
              if (event.event === "start") {
                return { ...prev, messages: prev.messages + `[开始测试] ${event.data}\n` };
              } else if (event.event === "toolcall") {
                return { ...prev, messages: prev.messages + `\n[调用工具] ${event.data}\n` };
              } else if (event.event === "toolres") {
                return { ...prev, messages: prev.messages + `\n[工具输出] ${event.data}\n` };
              } else if (event.event === "message") {
                if (event.data.trim()) {
                  return { ...prev, messages: prev.messages + `${event.data}` };
                }
              } else if (event.event === "error") {
                return { ...prev, success: false, messages: prev.messages + `\n[错误] ${event.data}\n` };
              } else if (event.event === "success") {
                return { ...prev, success: true, messages: prev.messages + `\n[成功] ${event.data}` };
              }
              return prev;
            });
          },
          onclose() {
            applyIfCurrent((prev) => ({ ...prev, loading: false }));
          },
          onerror(err) {
            // 取消不是失败：走 settleCancelled（不抹已出结论），保留半截日志、不设 error/success。
            if (isAbortError(err)) {
              settleCancelled();
            } else if (err instanceof UnauthorizedError) {
              // 登录过期不是失败：失效出口已处置（标记/清会话/跳转），面板只收场不写结论，
              // 不以「测试失败」掩盖掉线。
              applyIfCurrent((prev) => ({ ...prev, loading: false }));
            } else {
              applyIfCurrent((prev) => ({
                ...prev,
                loading: false,
                error: toErrorMessage(err, REACT_TEST_FALLBACK_ERROR),
                success: false,
              }));
            }
            throw err;
          },
        });
        // 取消主路径收场：真实库 abort = dispose + resolve，既不进 onerror 也不进 catch
        // （见测试 mock 顶部注释），故 await 正常返回后按本条流自己的 signal 快照补收场，
        // 判据取请求构建时刻的快照、不读处置时刻的全局状态。
        if (controller.signal.aborted) {
          settleCancelled();
        }
      } catch (err) {
        // 过期不是「测试失败」：失效出口已处置（标记/清会话/跳转），不写失败结论掩盖掉线。
        // 收场复位按当前性做：过期错误可能不经 onerror——发起测试瞬间无凭证时
        // requireAuthTokenOrRedirect 直抛到这里，loading 已置起，必须复位。
        if (err instanceof UnauthorizedError) {
          applyIfCurrent((prev) => ({ ...prev, loading: false }));
          return;
        }
        // 取消的防御性分支。真实库的取消主路径是 dispose + resolve（不进 catch、
        // 也不进 onerror），收场在上面 await 之后按 signal 快照补写。
        if (isAbortError(err)) {
          return;
        }
        // 失败结论按当前性写：旧流的迟到失败不得覆写已接管的新流（同 applyIfCurrent 口径）。
        // 结论取具体原因（与 onerror 分支同款），不再用字面量「测试失败」盖掉——面板靠
        // ExpandableError 自己按原文推断类别，写死一句通用话会让失败原因不可见。
        applyIfCurrent((prev) => ({
          ...prev,
          loading: false,
          error: toErrorMessage(err, REACT_TEST_FALLBACK_ERROR),
          success: false,
        }));
        console.error(err);
      } finally {
        // 仅当这条流仍被认作当前流时才解绑，避免把已交接给新流的 ref 清空。
        if (currentControllerRef.current === controller) {
          currentControllerRef.current = null;
        }
      }
    },
    [setReactTestResult]
  );

  // 测试类型 → 执行函数的分发表（OCP）：新增 TestType 时 Record 穷尽性会强制补一行，
  // 不再像旧 if/else 链那样把未知类型静默落进 structured_output 的 else 分支。
  // 表在回调内构造：外提成每次渲染新建的对象会击穿 useCallback 的依赖稳定。
  // mode 是正交维度：它进每个 runner 的入参，不进分发表，所以表的结构不因它变化。
  const executeTest = useCallback(async () => {
    if (!selectedTestId) return;
    const testRunnerByType: Record<TestType, (id: number, mode: TestMode) => Promise<unknown>> = {
      connectivity: connectivityTest.run,
      // ReAct 端点恒为流式（第三方 agent 库自己发流），模式开关对它没有意义，故忽略 mode。
      react: (id) => handleReactTest(id),
      structured_output: structuredTest.run,
      thinking: thinkingTest.run,
      vision: visionTest.run,
    };
    await testRunnerByType[testType](selectedTestId, testMode);
  }, [
    connectivityTest.run,
    handleReactTest,
    structuredTest.run,
    thinkingTest.run,
    visionTest.run,
    selectedTestId,
    testType,
    testMode,
  ]);

  const executeTestNow = useCallback(() => {
    void executeTest();
  }, [executeTest]);

  return {
    testResults: connectivityTest.results,
    structuredTestResults: structuredTest.results,
    thinkingTestResults: thinkingTest.results,
    visionTestResults: visionTest.results,
    handleTest,
    dialogClose,
    executeTestNow,
  };
}
