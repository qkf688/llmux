import { useCallback, useState } from "react";
import { toast } from "sonner";
import {
  batchDeleteModelProviders,
  batchUpdateModelProvidersCapabilities,
  batchUpdateModelProvidersStatus,
  testModelProvider,
  type ModelWithProvider,
} from "@/lib/api";
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

  const testSingleAssociationInBatch = useCallback(
    async (
      associationId: number,
      testing: Set<number>,
      signal: AbortSignal,
      counters: { success: number; failed: number }
    ) => {
      if (signal.aborted) {
        testing.delete(associationId);
        return;
      }

      try {
        setAssociationTestResults((prev) => ({ ...prev, [associationId]: { loading: true, success: null } }));

        await testModelProvider(associationId);

        setAssociationTestResults((prev) => ({ ...prev, [associationId]: { loading: false, success: true } }));

        counters.success++;

        setBatchTestProgress((prev) => ({
          ...prev,
          completed: prev.completed + 1,
          success: counters.success,
          testing: testing.size - 1,
        }));
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        setAssociationTestResults((prev) => ({
          ...prev,
          [associationId]: { loading: false, success: false, error: message },
        }));

        counters.failed++;

        setBatchTestProgress((prev) => ({
          ...prev,
          completed: prev.completed + 1,
          failed: counters.failed,
          testing: testing.size - 1,
        }));
      } finally {
        testing.delete(associationId);
      }
    },
    [setAssociationTestResults, setBatchTestProgress]
  );

  const startBatchTest = useCallback(
    async (associationIds: number[]) => {
      if (associationIds.length === 0) {
        toast.error("没有可测试的关联");
        return;
      }

      setBatchTesting(true);
      setBatchTestProgress({ total: associationIds.length, completed: 0, success: 0, failed: 0, testing: 0 });

      const abortController = new AbortController();
      setTestAbortController(abortController);

      const counters = { success: 0, failed: 0 };
      const concurrency = 3;
      const queue = [...associationIds];
      const testing = new Set<number>();
      const promises: Promise<void>[] = [];

      try {
        while (queue.length > 0 && !abortController.signal.aborted) {
          while (testing.size < concurrency && queue.length > 0) {
            const id = queue.shift()!;
            testing.add(id);

            setBatchTestProgress((prev) => ({ ...prev, testing: testing.size }));

            const testPromise = testSingleAssociationInBatch(id, testing, abortController.signal, counters);
            promises.push(testPromise);
          }

          await new Promise((resolve) => setTimeout(resolve, 100));
        }

        await Promise.all(promises);

        if (!abortController.signal.aborted) {
          toast.success(`批量测试完成：成功 ${counters.success} 个，失败 ${counters.failed} 个`, { duration: 5000 });
        }
      } finally {
        setBatchTesting(false);
        setTestAbortController(null);
      }
    },
    [setBatchTestProgress, setBatchTesting, testSingleAssociationInBatch]
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
    setBatchTestProgress({ total: 0, completed: 0, success: 0, failed: 0, testing: 0 });
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
