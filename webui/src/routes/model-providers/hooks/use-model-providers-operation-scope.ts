import { useCallback } from "react";
import { toast } from "sonner";
import { enableAllAssociations, resetModelPriorities, resetModelWeights } from "@/lib/api";

type UseModelProvidersOperationScopeInput = {
  selectedModelId: number | null;
  isGlobalScope: boolean;
  fetchModelProviders: (modelId: number) => Promise<void>;

  setResettingWeights: (resetting: boolean) => void;
  setResettingPriorities: (resetting: boolean) => void;
  setEnablingAssociations: (enabling: boolean) => void;
};

export function useModelProvidersOperationScope({
  selectedModelId,
  isGlobalScope,
  fetchModelProviders,
  setResettingWeights,
  setResettingPriorities,
  setEnablingAssociations,
}: UseModelProvidersOperationScopeInput) {
  const handleResetWeights = useCallback(async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setResettingWeights(true);
      const result = await resetModelWeights(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(
        result.updated > 0
          ? `已重置 ${result.updated} 个模型关联的权重到 ${result.default_weight}`
          : `所有模型关联已处于默认权重 ${result.default_weight}`
      );
      if (selectedModelId) {
        await fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`重置权重失败: ${message}`);
    } finally {
      setResettingWeights(false);
    }
  }, [fetchModelProviders, isGlobalScope, selectedModelId, setResettingWeights]);

  const handleResetPriorities = useCallback(async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setResettingPriorities(true);
      const result = await resetModelPriorities(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(
        result.updated > 0
          ? `已重置 ${result.updated} 个模型关联的优先级到 ${result.default_priority}`
          : `所有模型关联已处于默认优先级 ${result.default_priority}`
      );
      if (selectedModelId) {
        await fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`重置优先级失败: ${message}`);
    } finally {
      setResettingPriorities(false);
    }
  }, [fetchModelProviders, isGlobalScope, selectedModelId, setResettingPriorities]);

  const handleEnableAssociations = useCallback(async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setEnablingAssociations(true);
      const result = await enableAllAssociations(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(result.updated > 0 ? `已启用 ${result.updated} 个模型关联` : "所有模型关联已处于启用状态");
      if (selectedModelId) {
        await fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`启用关联失败: ${message}`);
    } finally {
      setEnablingAssociations(false);
    }
  }, [fetchModelProviders, isGlobalScope, selectedModelId, setEnablingAssociations]);

  return { handleResetWeights, handleResetPriorities, handleEnableAssociations };
}

