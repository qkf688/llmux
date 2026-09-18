import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import {
  testProviderModel,
  type Provider,
  type ProviderModel,
  type ProviderModelTestResult,
} from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import { copyTextWithToast } from "@/lib/clipboard";
import { isAbortError } from "@/lib/errors";
import { runConcurrentBatch, type BatchItemOutcome } from "@/lib/concurrent-batch";
import { DEFAULT_BATCH_TEST_PROGRESS, type BatchTestProgress, type ModelTestResult } from "../types";

type UseProviderModelTestingInput = {
  allModelsProvider: Provider | null;
  modelsOpenId: number | null;
  filteredAllModels: string[];
  filteredProviderModels: ProviderModel[];
  selectedAllModels: string[];
  selectedUpstreamModels: string[];
  allModelsTestResults: Record<string, ModelTestResult>;
  upstreamTestResults: Record<string, ModelTestResult>;
  setSelectedAllModels: (models: Updater<string[]>) => void;
  setSelectedUpstreamModels: (models: Updater<string[]>) => void;
  setAllModelsTestResults: (results: Updater<Record<string, ModelTestResult>>) => void;
  setUpstreamTestResults: (results: Updater<Record<string, ModelTestResult>>) => void;
  setBatchTesting: (testing: boolean) => void;
  setBatchTestProgress: (progress: Updater<BatchTestProgress>) => void;
  setUpstreamBatchTesting: (testing: boolean) => void;
  setUpstreamBatchTestProgress: (progress: Updater<BatchTestProgress>) => void;
};

export function useProviderModelTesting({
  allModelsProvider,
  modelsOpenId,
  filteredAllModels,
  filteredProviderModels,
  selectedAllModels,
  selectedUpstreamModels,
  allModelsTestResults,
  upstreamTestResults,
  setSelectedAllModels,
  setSelectedUpstreamModels,
  setAllModelsTestResults,
  setUpstreamTestResults,
  setBatchTesting,
  setBatchTestProgress,
  setUpstreamBatchTesting,
  setUpstreamBatchTestProgress,
}: UseProviderModelTestingInput) {
  const [testAbortController, setTestAbortController] = useState<AbortController | null>(null);
  const [upstreamTestAbortController, setUpstreamTestAbortController] = useState<AbortController | null>(null);

  // 切走页面（组件卸载）时中止在飞的批量：controller 用 ref 镜像一份，cleanup 才拿得到最新值
  // （effect 依赖为空，闭包里的 state 会停在首次渲染的 null）。
  const testControllerRef = useRef<AbortController | null>(null);
  const upstreamControllerRef = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      testControllerRef.current?.abort();
      upstreamControllerRef.current?.abort();
    },
    []
  );

  const copyModelName = async (modelName: string) => {
    await copyTextWithToast(modelName, `已复制模型名称: ${modelName}`);
  };

  /**
   * #19 起「一次失败的测试」也是 HTTP 200 + code 200，结论只在 `passed`（后端同时回传
   * 实际命中的分组，供 C2 形态把 ✕ 归位到徽标）。所以不能只靠 apiRequest 抛错判定——
   * 那样失败会被当成通过。抛错仍是传输/参数层问题，走 catch 分支。
   */
  const toTestOutcome = (result: ProviderModelTestResult): { success: boolean; error?: string } =>
    result?.passed === true ? { success: true } : { success: false, error: result?.error || "测试未通过" };

  /** C2 结果键：`${model}::${groupId}`，groupId=0 表示自动口径（行尾按钮）。 */
  const testResultKey = (modelName: string, groupId?: number) => `${modelName}::${groupId ?? 0}`;

  /** 从结果键拆出模型名（select 成功/失败用；只认自动口径 `::0`，分组级结果是诊断不驱动选中）。 */
  const modelFromKey = (key: string) => key.slice(0, key.lastIndexOf("::"));

  /** 只取自动口径条目，供「选择成功/失败模型」使用。 */
  const autoResults = (results: Record<string, ModelTestResult>) =>
    Object.entries(results).filter(([key]) => key.endsWith("::0"));

  const handleTestAllModel = async (
    modelName: string,
    groupId?: number,
    signal?: AbortSignal
  ): Promise<BatchItemOutcome> => {
    if (!allModelsProvider) return "failed";
    const key = testResultKey(modelName, groupId);
    setAllModelsTestResults((prev) => ({
      ...prev,
      [key]: { loading: true, success: null },
    }));
    try {
      const result = await testProviderModel(allModelsProvider.ID, modelName, groupId, signal);
      const outcome = toTestOutcome(result);
      setAllModelsTestResults((prev) => {
        const next = { ...prev, [key]: { loading: false, ...outcome } };
        // 自动口径：把结果同步落到后端回传的实际分组徽标上
        if (!groupId && result?.group_id) {
          next[testResultKey(modelName, result.group_id)] = { loading: false, ...outcome };
        }
        return next;
      });
      return outcome.success ? "success" : "failed";
    } catch (err) {
      // 中止不是失败：标 cancelled 且不进失败计数，否则「取消整批」会把每个在飞的项
      // 都算成「这个模型不可用」。
      if (isAbortError(err)) {
        setAllModelsTestResults((prev) => ({
          ...prev,
          [key]: { loading: false, success: null, cancelled: true },
        }));
        return "cancelled";
      }
      const message = err instanceof Error ? err.message : String(err);
      setAllModelsTestResults((prev) => ({
        ...prev,
        [key]: { loading: false, success: false, error: message },
      }));
      return "failed";
    }
  };

  const selectAllSuccessful = () => {
    const successfulModels = [
      ...new Set(
        autoResults(allModelsTestResults)
          .filter(([, result]) => result.success === true)
          .map(([key]) => modelFromKey(key)),
      ),
    ];

    if (successfulModels.length === 0) {
      toast.info("当前没有测试成功的模型");
      return;
    }

    setSelectedAllModels(successfulModels);
    toast.success(`已选择 ${successfulModels.length} 个测试成功的模型`);
  };

  const selectAllFailed = () => {
    const failedModels = [
      ...new Set(
        autoResults(allModelsTestResults)
          .filter(([, result]) => result.success === false)
          .map(([key]) => modelFromKey(key)),
      ),
    ];

    if (failedModels.length === 0) {
      toast.info("当前没有测试失败的模型");
      return;
    }

    setSelectedAllModels(failedModels);
    toast.success(`已选择 ${failedModels.length} 个测试失败的模型`);
  };

  const startBatchTest = async (models: string[]) => {
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }

    setBatchTesting(true);
    setBatchTestProgress({
      ...DEFAULT_BATCH_TEST_PROGRESS,
      total: models.length,
    });

    const abortController = new AbortController();
    setTestAbortController(abortController);
    testControllerRef.current = abortController;

    try {
      const summary = await runConcurrentBatch({
        items: models,
        signal: abortController.signal,
        // signal 一路透传到 fetch：只判 aborted 不传的话，在飞的那一项仍会跑完并计费。
        runItem: (model, signal) => handleTestAllModel(model, undefined, signal),
        onQueueChange: (testing) => {
          setBatchTestProgress((prev) => ({
            ...prev,
            testing,
          }));
        },
        onItemDone: ({ completed, successCount, failedCount, cancelledCount, testing }) => {
          setBatchTestProgress((prev) => ({
            ...prev,
            completed,
            success: successCount,
            failed: failedCount,
            cancelled: cancelledCount,
            testing,
          }));
        },
      });

      // 取消路径由 handleCancelBatchTest 即时 toast，这里不再重复；被取消的项数在进度条上可见。
      if (!summary.aborted) {
        toast.success(`批量测试完成：成功 ${summary.success} 个，失败 ${summary.failed} 个`, { duration: 5000 });
      }
    } finally {
      setBatchTesting(false);
      setTestAbortController(null);
      testControllerRef.current = null;
    }
  };

  const handleBatchTestAll = async () => {
    const models = filteredAllModels;
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }
    await startBatchTest(models);
  };

  const handleBatchTestSelected = async () => {
    if (selectedAllModels.length === 0) {
      toast.error("请先选择要测试的模型");
      return;
    }
    await startBatchTest(selectedAllModels);
  };

  const handleCancelBatchTest = () => {
    if (testAbortController) {
      testAbortController.abort();
      toast.info("已取消批量测试");
    }
  };

  const handleTestUpstreamModel = async (
    modelName: string,
    groupId?: number,
    signal?: AbortSignal
  ): Promise<BatchItemOutcome> => {
    if (!modelsOpenId) return "failed";
    const key = testResultKey(modelName, groupId);
    setUpstreamTestResults((prev) => ({
      ...prev,
      [key]: { loading: true, success: null },
    }));
    try {
      const result = await testProviderModel(modelsOpenId, modelName, groupId, signal);
      const outcome = toTestOutcome(result);
      setUpstreamTestResults((prev) => {
        const next = { ...prev, [key]: { loading: false, ...outcome } };
        // 自动口径：把结果同步落到后端回传的实际分组徽标上
        if (!groupId && result?.group_id) {
          next[testResultKey(modelName, result.group_id)] = { loading: false, ...outcome };
        }
        return next;
      });
      return outcome.success ? "success" : "failed";
    } catch (err) {
      if (isAbortError(err)) {
        setUpstreamTestResults((prev) => ({
          ...prev,
          [key]: { loading: false, success: null, cancelled: true },
        }));
        return "cancelled";
      }
      const message = err instanceof Error ? err.message : String(err);
      setUpstreamTestResults((prev) => ({
        ...prev,
        [key]: { loading: false, success: false, error: message },
      }));
      return "failed";
    }
  };

  const startUpstreamBatchTest = async (models: string[]) => {
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }

    setUpstreamBatchTesting(true);
    setUpstreamBatchTestProgress({
      ...DEFAULT_BATCH_TEST_PROGRESS,
      total: models.length,
    });

    const abortController = new AbortController();
    setUpstreamTestAbortController(abortController);
    upstreamControllerRef.current = abortController;

    try {
      const summary = await runConcurrentBatch({
        items: models,
        signal: abortController.signal,
        runItem: (model, signal) => handleTestUpstreamModel(model, undefined, signal),
        onQueueChange: (testing) => {
          setUpstreamBatchTestProgress((prev) => ({
            ...prev,
            testing,
          }));
        },
        onItemDone: ({ completed, successCount, failedCount, cancelledCount, testing }) => {
          setUpstreamBatchTestProgress((prev) => ({
            ...prev,
            completed,
            success: successCount,
            failed: failedCount,
            cancelled: cancelledCount,
            testing,
          }));
        },
      });

      if (!summary.aborted) {
        toast.success(`批量测试完成：成功 ${summary.success} 个，失败 ${summary.failed} 个`, { duration: 5000 });
      }
    } finally {
      setUpstreamBatchTesting(false);
      setUpstreamTestAbortController(null);
      upstreamControllerRef.current = null;
    }
  };

  const handleBatchTestUpstreamAll = async () => {
    const models = filteredProviderModels.map((model) => model.id);
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }
    await startUpstreamBatchTest(models);
  };

  const handleBatchTestUpstreamSelected = async () => {
    if (selectedUpstreamModels.length === 0) {
      toast.error("请先选择要测试的模型");
      return;
    }
    await startUpstreamBatchTest(selectedUpstreamModels);
  };

  const handleCancelUpstreamBatchTest = () => {
    if (upstreamTestAbortController) {
      upstreamTestAbortController.abort();
      toast.info("已取消批量测试");
    }
  };

  const selectUpstreamSuccessful = () => {
    const successfulModels = [
      ...new Set(
        autoResults(upstreamTestResults)
          .filter(([, result]) => result.success === true)
          .map(([key]) => modelFromKey(key)),
      ),
    ];

    if (successfulModels.length === 0) {
      toast.info("当前没有测试成功的模型");
      return;
    }

    setSelectedUpstreamModels(successfulModels);
    toast.success(`已选择 ${successfulModels.length} 个测试成功的模型`);
  };

  const selectUpstreamFailed = () => {
    const failedModels = [
      ...new Set(
        autoResults(upstreamTestResults)
          .filter(([, result]) => result.success === false)
          .map(([key]) => modelFromKey(key)),
      ),
    ];

    if (failedModels.length === 0) {
      toast.info("当前没有测试失败的模型");
      return;
    }

    setSelectedUpstreamModels(failedModels);
    toast.success(`已选择 ${failedModels.length} 个测试失败的模型`);
  };

  const clearAllTestResults = () => {
    setAllModelsTestResults({});
  };

  const clearUpstreamTestResults = () => {
    setUpstreamTestResults({});
  };

  return {
    copyModelName,
    handleTestAllModel,
    selectAllSuccessful,
    selectAllFailed,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    clearAllTestResults,
    handleTestUpstreamModel,
    handleBatchTestUpstreamAll,
    handleBatchTestUpstreamSelected,
    handleCancelUpstreamBatchTest,
    clearUpstreamTestResults,
    selectUpstreamSuccessful,
    selectUpstreamFailed,
  };
}

