import { useCallback, useRef, useState } from "react";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { getAuthToken } from "@/stores/auth";
import {
  testModelProvider,
  testModelProviderStructuredOutput,
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
      setTestResults((prev) => ({ ...prev, [id]: { loading: false, result: { error: "测试失败" + err } } }));
      console.error(err);
      return { error: "测试失败" + err };
    }
  }, []);

  const handleStructuredOutputTest = useCallback(async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setStructuredTestResults((prev) => ({ ...prev, [id]: { loading: true, result: null } }));
      const result = await testModelProviderStructuredOutput(id);
      setStructuredTestResults((prev) => ({ ...prev, [id]: { loading: false, result } }));
      return result;
    } catch (err) {
      setStructuredTestResults((prev) => ({
        ...prev,
        [id]: { loading: false, result: { passed: false, error: "测试失败" + err } },
      }));
      console.error(err);
      return { passed: false, error: "测试失败" + err };
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

  const executeTest = useCallback(async () => {
    if (!selectedTestId) return;

    if (testType === "connectivity") {
      await handleConnectivityTest(selectedTestId);
    } else if (testType === "react") {
      await handleReactTest(selectedTestId);
    } else {
      await handleStructuredOutputTest(selectedTestId);
    }
  }, [handleConnectivityTest, handleReactTest, handleStructuredOutputTest, selectedTestId, testType]);

  const executeTestNow = useCallback(() => {
    void executeTest();
  }, [executeTest]);

  return { testResults, structuredTestResults, handleTest, dialogClose, executeTestNow };
}
