import { useCallback, useRef, useState } from "react";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { getAuthToken } from "@/stores/auth";
import { toErrorMessage } from "@/lib/errors";
import {
  testModelProvider,
  testModelProviderStructuredOutput,
  testModelProviderThinking,
  testModelProviderVision,
  type ModelProviderTestResult,
} from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import type { CapabilityTestKey, TestMode, TestType } from "../types";

type ReactTestResult = {
  loading: boolean;
  messages: string;
  success: boolean | null;
  error: string | null;
};

type CapabilityTestEntry = { loading: boolean; result: ModelProviderTestResult | null };

export type CapabilityTestResults = Record<CapabilityTestKey, CapabilityTestEntry>;

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersTestingInput = {
  setTestDialogOpen: (open: boolean) => void;
  setSelectedTestId: (id: number) => void;
  setTestType: (value: TestType) => void;
  setTestMode: (value: TestMode) => void;
  setReactTestResult: Setter<ReactTestResult>;

  selectedTestId: number | null;
  testType: TestType;
  testMode: TestMode;
};

// 结果存储键：关联 id + 模式。同一个关联的流式与非流式结论都要看得见
// （「非流式通过、流式失败」正是本次要检出的缺陷），共用一格会让切换模式后读到旧结论。
export function capabilityTestKey(id: number, mode: TestMode): CapabilityTestKey {
  return `${id}:${mode}`;
}

// JSON 型能力测试执行单元：按「关联 + 模式」记 loading/结果 + 单次执行与异常兜底。
// 此前 connectivity/structured/thinking/vision 各抄一份同款 try/catch（第 5 类测试就要抄第 5 份），
// 收敛后新增测试类型只需多调一次本 hook（OCP）。api 传模块级函数引用，天然稳定。
function useCapabilityTest(api: (id: number, mode: TestMode) => Promise<ModelProviderTestResult>) {
  const [results, setResults] = useState<CapabilityTestResults>({});

  const run = useCallback(
    async (id: number, mode: TestMode): Promise<ModelProviderTestResult> => {
      const key = capabilityTestKey(id, mode);
      try {
        setResults((prev) => ({ ...prev, [key]: { loading: true, result: null } }));
        const result = await api(id, mode);
        setResults((prev) => ({ ...prev, [key]: { loading: false, result } }));
        return result;
      } catch (err) {
        const message = `测试失败: ${toErrorMessage(err)}`;
        setResults((prev) => ({ ...prev, [key]: { loading: false, result: { passed: false, error: message } } }));
        console.error(err);
        return { passed: false, error: message };
      }
    },
    [api]
  );

  return { results, run };
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

  const currentControllerRef = useRef<AbortController | null>(null);

  const dialogClose = useCallback(() => {
    setTestDialogOpen(false);
  }, [setTestDialogOpen]);

  const handleTest = useCallback(
    (id: number) => {
      currentControllerRef.current?.abort();
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
      setReactTestResult((prev) => ({ ...prev, messages: "", loading: true }));
      try {
        const token = getAuthToken();
        if (!token) {
          window.location.href = "/login";
          return;
        }

        const controller = new AbortController();
        currentControllerRef.current = controller;

        await fetchEventSource(`/api/test/react/${id}`, {
          method: "GET",
          headers: { Authorization: `Bearer ${token}` },
          signal: controller.signal,
          onmessage(event) {
            setReactTestResult((prev) => {
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
            setReactTestResult((prev) => ({ ...prev, loading: false }));
          },
          onerror(err) {
            setReactTestResult((prev) => ({
              ...prev,
              loading: false,
              error: err.message || "测试过程中发生错误",
              success: false,
            }));
            throw err;
          },
        });
      } catch (err) {
        setReactTestResult((prev) => ({ ...prev, loading: false, error: "测试失败", success: false }));
        console.error(err);
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
