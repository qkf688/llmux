import { useState } from "react";
import { toast } from "sonner";
import { testProviderModel, type Provider, type ProviderModel } from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import { copyTextDetailed } from "../utils/clipboard";
import { runConcurrentBatch } from "../utils/batch-test";
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

  const copyModelName = async (modelName: string) => {
    const result = await copyTextDetailed(modelName, { showManualPrompt: true });
    if (result.ok) {
      toast.success(`已复制模型名称: ${modelName}`);
      return;
    }

    if (result.method === "manual_prompt") {
      toast.info("浏览器限制：通过 IP + HTTP 访问时通常无法自动复制，已弹出手动复制窗口；建议改用 HTTPS/域名访问。");
      return;
    } else {
      toast.error("复制失败：当前环境不支持自动复制，请手动复制（建议使用 HTTPS/域名访问）。");
    }
  };

  const handleTestAllModel = async (modelName: string): Promise<boolean> => {
    if (!allModelsProvider) return false;
    setAllModelsTestResults((prev) => ({
      ...prev,
      [modelName]: { loading: true, success: null },
    }));
    try {
      await testProviderModel(allModelsProvider.ID, modelName);
      setAllModelsTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: true },
      }));
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setAllModelsTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: false, error: message },
      }));
      return false;
    }
  };

  const selectAllSuccessful = () => {
    const successfulModels = Object.entries(allModelsTestResults)
      .filter(([, result]) => result.success === true)
      .map(([modelName]) => modelName);

    if (successfulModels.length === 0) {
      toast.info("当前没有测试成功的模型");
      return;
    }

    setSelectedAllModels(successfulModels);
    toast.success(`已选择 ${successfulModels.length} 个测试成功的模型`);
  };

  const selectAllFailed = () => {
    const failedModels = Object.entries(allModelsTestResults)
      .filter(([, result]) => result.success === false)
      .map(([modelName]) => modelName);

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

    try {
      const summary = await runConcurrentBatch({
        items: models,
        signal: abortController.signal,
        runItem: (model, signal) => {
          if (signal.aborted) {
            return Promise.resolve(false);
          }
          return handleTestAllModel(model);
        },
        onQueueChange: (testing) => {
          setBatchTestProgress((prev) => ({
            ...prev,
            testing,
          }));
        },
        onItemDone: ({ completed, successCount, failedCount, testing }) => {
          setBatchTestProgress((prev) => ({
            ...prev,
            completed,
            success: successCount,
            failed: failedCount,
            testing,
          }));
        },
      });

      if (!summary.aborted) {
        toast.success(`批量测试完成：成功 ${summary.success} 个，失败 ${summary.failed} 个`, { duration: 5000 });
      }
    } finally {
      setBatchTesting(false);
      setTestAbortController(null);
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

  const handleTestUpstreamModel = async (modelName: string): Promise<boolean> => {
    if (!modelsOpenId) return false;
    setUpstreamTestResults((prev) => ({
      ...prev,
      [modelName]: { loading: true, success: null },
    }));
    try {
      await testProviderModel(modelsOpenId, modelName);
      setUpstreamTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: true },
      }));
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setUpstreamTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: false, error: message },
      }));
      return false;
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

    try {
      const summary = await runConcurrentBatch({
        items: models,
        signal: abortController.signal,
        runItem: (model, signal) => {
          if (signal.aborted) {
            return Promise.resolve(false);
          }
          return handleTestUpstreamModel(model);
        },
        onQueueChange: (testing) => {
          setUpstreamBatchTestProgress((prev) => ({
            ...prev,
            testing,
          }));
        },
        onItemDone: ({ completed, successCount, failedCount, testing }) => {
          setUpstreamBatchTestProgress((prev) => ({
            ...prev,
            completed,
            success: successCount,
            failed: failedCount,
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
    const successfulModels = Object.entries(upstreamTestResults)
      .filter(([, result]) => result.success === true)
      .map(([modelName]) => modelName);

    if (successfulModels.length === 0) {
      toast.info("当前没有测试成功的模型");
      return;
    }

    setSelectedUpstreamModels(successfulModels);
    toast.success(`已选择 ${successfulModels.length} 个测试成功的模型`);
  };

  const selectUpstreamFailed = () => {
    const failedModels = Object.entries(upstreamTestResults)
      .filter(([, result]) => result.success === false)
      .map(([modelName]) => modelName);

    if (failedModels.length === 0) {
      toast.info("当前没有测试失败的模型");
      return;
    }

    setSelectedUpstreamModels(failedModels);
    toast.success(`已选择 ${failedModels.length} 个测试失败的模型`);
  };

  return {
    copyModelName,
    handleTestAllModel,
    selectAllSuccessful,
    selectAllFailed,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    handleTestUpstreamModel,
    handleBatchTestUpstreamAll,
    handleBatchTestUpstreamSelected,
    handleCancelUpstreamBatchTest,
    selectUpstreamSuccessful,
    selectUpstreamFailed,
  };
}

