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
import type { TestType } from "../types";

type ReactTestResult = {
  loading: boolean;
  messages: string;
  success: boolean | null;
  error: string | null;
};

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersTestingInput = {
  setTestDialogOpen: (open: boolean) => void;
  setSelectedTestId: (id: number) => void;
  setTestType: (value: TestType) => void;
  setReactTestResult: Setter<ReactTestResult>;

  selectedTestId: number | null;
  testType: TestType;
};

export function useModelProvidersTesting({
  setTestDialogOpen,
  setSelectedTestId,
  setTestType,
  setReactTestResult,
  selectedTestId,
  testType,
}: UseModelProvidersTestingInput) {
  const [testResults, setTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [structuredTestResults, setStructuredTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [thinkingTestResults, setThinkingTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [visionTestResults, setVisionTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});

  const currentControllerRef = useRef<AbortController | null>(null);

  const dialogClose = useCallback(() => {
    setTestDialogOpen(false);
  }, [setTestDialogOpen]);

  const handleTest = useCallback(
    (id: number) => {
      currentControllerRef.current?.abort();
      setSelectedTestId(id);
      setTestType("connectivity");
      setTestDialogOpen(true);
      setReactTestResult({
        loading: false,
        messages: "",
        success: null,
        error: null,
      });
    },
    [setReactTestResult, setSelectedTestId, setTestDialogOpen, setTestType]
  );

  const handleConnectivityTest = useCallback(async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setTestResults((prev) => ({ ...prev, [id]: { loading: true, result: null } }));
      const result = await testModelProvider(id);
      setTestResults((prev) => ({ ...prev, [id]: { loading: false, result } }));
      return result;
    } catch (err) {
      const message = `测试失败: ${toErrorMessage(err)}`;
      setTestResults((prev) => ({ ...prev, [id]: { loading: false, result: { error: message } } }));
      console.error(err);
      return { error: message };
    }
  }, []);

  const handleStructuredOutputTest = useCallback(async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setStructuredTestResults((prev) => ({ ...prev, [id]: { loading: true, result: null } }));
      const result = await testModelProviderStructuredOutput(id);
      setStructuredTestResults((prev) => ({ ...prev, [id]: { loading: false, result } }));
      return result;
    } catch (err) {
      const message = `测试失败: ${toErrorMessage(err)}`;
      setStructuredTestResults((prev) => ({
        ...prev,
        [id]: { loading: false, result: { passed: false, error: message } },
      }));
      console.error(err);
      return { passed: false, error: message };
    }
  }, []);

  const handleThinkingTest = useCallback(async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setThinkingTestResults((prev) => ({ ...prev, [id]: { loading: true, result: null } }));
      const result = await testModelProviderThinking(id);
      setThinkingTestResults((prev) => ({ ...prev, [id]: { loading: false, result } }));
      return result;
    } catch (err) {
      const message = `测试失败: ${toErrorMessage(err)}`;
      setThinkingTestResults((prev) => ({
        ...prev,
        [id]: { loading: false, result: { passed: false, error: message } },
      }));
      console.error(err);
      return { passed: false, error: message };
    }
  }, []);

  const handleVisionTest = useCallback(async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setVisionTestResults((prev) => ({ ...prev, [id]: { loading: true, result: null } }));
      const result = await testModelProviderVision(id);
      setVisionTestResults((prev) => ({ ...prev, [id]: { loading: false, result } }));
      return result;
    } catch (err) {
      const message = `测试失败: ${toErrorMessage(err)}`;
      setVisionTestResults((prev) => ({
        ...prev,
        [id]: { loading: false, result: { passed: false, error: message } },
      }));
      console.error(err);
      return { passed: false, error: message };
    }
  }, []);

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
  const executeTest = useCallback(async () => {
    if (!selectedTestId) return;
    const testRunnerByType: Record<TestType, (id: number) => Promise<unknown>> = {
      connectivity: handleConnectivityTest,
      react: handleReactTest,
      structured_output: handleStructuredOutputTest,
      thinking: handleThinkingTest,
      vision: handleVisionTest,
    };
    await testRunnerByType[testType](selectedTestId);
  }, [handleConnectivityTest, handleReactTest, handleStructuredOutputTest, handleThinkingTest, handleVisionTest, selectedTestId, testType]);

  const executeTestNow = useCallback(() => {
    void executeTest();
  }, [executeTest]);

  return {
    testResults,
    structuredTestResults,
    thinkingTestResults,
    visionTestResults,
    handleTest,
    dialogClose,
    executeTestNow,
  };
}
