import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import {
  batchDeleteModelProviders,
  batchUpdateModelProvidersCapabilities,
  batchUpdateModelProvidersStatus,
  testModelProvider,
  type ModelWithProvider,
} from "@/lib/api";
import { isAbortError } from "@/lib/errors";
import { UnauthorizedError } from "@/lib/api/core/client";
import { runConcurrentBatch, type BatchItemOutcome } from "@/lib/concurrent-batch";
import type { Updater } from "@/stores/core/updater";
import type { AssociationBatchTestResult, BatchTestProgress } from "../types";

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersBatchInput = {
  selectedModelId: number | null;
  fetchModelProviders: (modelId: number) => Promise<void>;

  filteredModelProviders: ModelWithProvider[];
  selectedAssociationIds: number[];
  setSelectedAssociationIds: Setter<number[]>;

  setModelProviders: Setter<ModelWithProvider[]>;

  setBatchDeleteDialogOpen: (open: boolean) => void;
  setBatchDeleting: (deleting: boolean) => void;
  setBatchUpdatingStatus: (updating: boolean) => void;
  setBatchCapabilitiesDialogOpen: (open: boolean) => void;
  setBatchUpdatingCapabilities: (updating: boolean) => void;

  setBatchTesting: (testing: boolean) => void;
  setBatchTestProgress: Setter<BatchTestProgress>;

  associationTestResults: Record<number, AssociationBatchTestResult>;
  setAssociationTestResults: Setter<Record<number, AssociationBatchTestResult>>;
};

export function useModelProvidersBatch({
  selectedModelId,
  fetchModelProviders,
  filteredModelProviders,
  selectedAssociationIds,
  setSelectedAssociationIds,
  setModelProviders,
  setBatchDeleteDialogOpen,
  setBatchDeleting,
  setBatchUpdatingStatus,
  setBatchCapabilitiesDialogOpen,
  setBatchUpdatingCapabilities,
  setBatchTesting,
  setBatchTestProgress,
  associationTestResults,
  setAssociationTestResults,
}: UseModelProvidersBatchInput) {
  const [testAbortController, setTestAbortController] = useState<AbortController | null>(null);

  // 卸载（切走页面）时中止在飞的批量：ref 镜像一份，cleanup 才拿得到最新值（effect 无依赖）。
  const testControllerRef = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      testControllerRef.current?.abort();
    },
    []
  );

  const handleSelectAllAssociations = useCallback(
    (checked: boolean) => {
      if (checked) {
        setSelectedAssociationIds(filteredModelProviders.map((mp) => mp.ID));
      } else {
        setSelectedAssociationIds([]);
      }
    },
    [filteredModelProviders, setSelectedAssociationIds]
  );

  const handleSelectOneAssociation = useCallback(
    (id: number, checked: boolean) => {
      if (checked) {
        setSelectedAssociationIds([...selectedAssociationIds, id]);
      } else {
        setSelectedAssociationIds(selectedAssociationIds.filter((selectedId) => selectedId !== id));
      }
    },
    [selectedAssociationIds, setSelectedAssociationIds]
  );

  const handleBatchDeleteAssociations = useCallback(async () => {
    if (selectedAssociationIds.length === 0) return;
    setBatchDeleting(true);
    try {
      const result = await batchDeleteModelProviders(selectedAssociationIds);
      toast.success(`成功删除 ${result.deleted} 个关联`);
      setSelectedAssociationIds([]);
      setBatchDeleteDialogOpen(false);
      if (selectedModelId) {
        await fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`批量删除关联失败: ${message}`);
    } finally {
      setBatchDeleting(false);
    }
  }, [
    fetchModelProviders,
    selectedAssociationIds,
    selectedModelId,
    setBatchDeleteDialogOpen,
    setBatchDeleting,
    setSelectedAssociationIds,
  ]);

  const handleBatchUpdateStatus = useCallback(
    async (status: boolean) => {
      if (selectedAssociationIds.length === 0) {
        toast.error("请先选择要操作的关联");
        return;
      }

      setBatchUpdatingStatus(true);
      try {
        const result = await batchUpdateModelProvidersStatus(selectedAssociationIds, status);
        toast.success(`成功${status ? "启用" : "停用"} ${result.updated} 个关联`);

        setModelProviders((prev) =>
          prev.map((item) => (selectedAssociationIds.includes(item.ID) ? { ...item, Status: status } : item))
        );
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`批量${status ? "启用" : "停用"}失败: ${message}`);
      } finally {
        setBatchUpdatingStatus(false);
      }
    },
    [selectedAssociationIds, setBatchUpdatingStatus, setModelProviders]
  );

  const handleBatchUpdateCapabilities = useCallback(
    async (capabilities: { tool_call?: boolean; structured_output?: boolean; image?: boolean }) => {
      if (selectedAssociationIds.length === 0) {
        toast.error("请先选择要操作的关联");
        return;
      }
      if (
        capabilities.tool_call === undefined &&
        capabilities.structured_output === undefined &&
        capabilities.image === undefined
      ) {
        toast.error("请选择至少一个能力字段进行更新");
        return;
      }

      setBatchUpdatingCapabilities(true);
      try {
        const result = await batchUpdateModelProvidersCapabilities(selectedAssociationIds, capabilities);
        toast.success(`成功更新 ${result.updated} 个关联能力`);

        setModelProviders((prev) =>
          prev.map((item) => {
            if (!selectedAssociationIds.includes(item.ID)) return item;
            return {
              ...item,
              ToolCall: capabilities.tool_call ?? item.ToolCall,
              StructuredOutput: capabilities.structured_output ?? item.StructuredOutput,
              Image: capabilities.image ?? item.Image,
            };
          })
        );

        setBatchCapabilitiesDialogOpen(false);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`批量更新能力失败: ${message}`);
      } finally {
        setBatchUpdatingCapabilities(false);
      }
    },
    [
      selectedAssociationIds,
      setBatchCapabilitiesDialogOpen,
      setBatchUpdatingCapabilities,
      setModelProviders,
    ]
  );

  // 单项执行单元：结果落表 + 中止/掉线与失败分流（非失败终止标 cancelled、不进失败计数）。
  // 调度交给共享的 runConcurrentBatch，这里只负责「跑一项并回一个结果」。
  const testSingleAssociation = useCallback(
    async (associationId: number, signal: AbortSignal): Promise<BatchItemOutcome> => {
      setAssociationTestResults((prev) => ({ ...prev, [associationId]: { loading: true, success: null } }));

      try {
        const result = await testModelProvider(associationId, "non_stream", signal);
        if (result?.error) {
          throw new Error(result.error);
        }

        setAssociationTestResults((prev) => ({ ...prev, [associationId]: { loading: false, success: true } }));
        return "success";
      } catch (error) {
        // 登录过期与中止同收场：失效出口已处置（标记过期/清会话/跳登录页），这一项根本没测成，
        // 记成「失败」等于告诉用户「这些关联坏了」——而它们一句请求都没发出去。与单测、
        // JSON 型能力测试、SSE 三路同口径（见 docs/architecture/modules/webui.md 401 处置节）。
        if (isAbortError(error) || error instanceof UnauthorizedError) {
          setAssociationTestResults((prev) => ({
            ...prev,
            [associationId]: { loading: false, success: null, cancelled: true },
          }));
          return "cancelled";
        }
        const message = error instanceof Error ? error.message : String(error);
        setAssociationTestResults((prev) => ({
          ...prev,
          [associationId]: { loading: false, success: false, error: message },
        }));
        return "failed";
      }
    },
    [setAssociationTestResults]
  );

  const startBatchTest = useCallback(
    async (associationIds: number[]) => {
      if (associationIds.length === 0) {
        toast.error("没有可测试的关联");
        return;
      }

      setBatchTesting(true);
      setBatchTestProgress({
        total: associationIds.length,
        completed: 0,
        success: 0,
        failed: 0,
        cancelled: 0,
        testing: 0,
      });

      const abortController = new AbortController();
      setTestAbortController(abortController);
      testControllerRef.current = abortController;

      try {
        const summary = await runConcurrentBatch({
          items: associationIds,
          signal: abortController.signal,
          // signal 透传到 fetch：只判 aborted 不传的话，在飞的那一项仍会跑完并计费。
          runItem: (id, signal) => testSingleAssociation(id, signal),
          onQueueChange: (testing) => {
            setBatchTestProgress((prev) => ({ ...prev, testing }));
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

        // 取消路径由 handleCancelBatchTest 即时 toast，这里不再重复；被取消项数在进度卡上可见。
        // 已取消数也要进汇总：登录过期整批收场时，若只报「失败 0 个」等于没交代去向。
        if (!summary.aborted) {
          const cancelledNote = summary.cancelled > 0 ? `，已取消 ${summary.cancelled} 个` : "";
          toast.success(
            `批量测试完成：成功 ${summary.success} 个，失败 ${summary.failed} 个${cancelledNote}`,
            { duration: 5000 },
          );
        }
      } finally {
        setBatchTesting(false);
        setTestAbortController(null);
        testControllerRef.current = null;
      }
    },
    [setBatchTestProgress, setBatchTesting, testSingleAssociation]
  );

  const handleBatchTestAll = useCallback(async () => {
    const ids = filteredModelProviders.map((mp) => mp.ID);
    await startBatchTest(ids);
  }, [filteredModelProviders, startBatchTest]);

  const handleBatchTestSelected = useCallback(async () => {
    if (selectedAssociationIds.length === 0) {
      toast.error("请先选择要测试的关联");
      return;
    }
    await startBatchTest(selectedAssociationIds);
  }, [selectedAssociationIds, startBatchTest]);

  const handleCancelBatchTest = useCallback(() => {
    if (testAbortController) {
      testAbortController.abort();
      toast.info("已取消批量测试");
    }
  }, [testAbortController]);

  const selectAllSuccessful = useCallback(() => {
    const visibleIds = new Set(filteredModelProviders.map((mp) => mp.ID));
    const successfulIds = Object.entries(associationTestResults)
      .filter(([id, result]) => result.success === true && visibleIds.has(Number.parseInt(id, 10)))
      .map(([id]) => Number.parseInt(id, 10));

    if (successfulIds.length === 0) {
      toast.info("当前列表中没有测试成功的项");
      return;
    }

    setSelectedAssociationIds(successfulIds);
    toast.success(`已选择 ${successfulIds.length} 个测试成功的项`);
  }, [associationTestResults, filteredModelProviders, setSelectedAssociationIds]);

  const selectAllFailed = useCallback(() => {
    const visibleIds = new Set(filteredModelProviders.map((mp) => mp.ID));
    const failedIds = Object.entries(associationTestResults)
      .filter(([id, result]) => result.success === false && visibleIds.has(Number.parseInt(id, 10)))
      .map(([id]) => Number.parseInt(id, 10));

    if (failedIds.length === 0) {
      toast.info("当前列表中没有测试失败的项");
      return;
    }

    setSelectedAssociationIds(failedIds);
    toast.success(`已选择 ${failedIds.length} 个测试失败的项`);
  }, [associationTestResults, filteredModelProviders, setSelectedAssociationIds]);

  const clearBatchTestResults = useCallback(() => {
    setBatchTestProgress({ total: 0, completed: 0, success: 0, failed: 0, cancelled: 0, testing: 0 });
    setAssociationTestResults({});
    toast.info("已清除测试结果");
  }, [setAssociationTestResults, setBatchTestProgress]);

  return {
    handleSelectAllAssociations,
    handleSelectOneAssociation,
    handleBatchDeleteAssociations,
    handleBatchUpdateStatus,
    handleBatchUpdateCapabilities,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    selectAllSuccessful,
    selectAllFailed,
    clearBatchTestResults,
  };
}
