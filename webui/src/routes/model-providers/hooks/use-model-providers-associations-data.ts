import { useCallback, useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { ModelWithProvider } from "@/lib/api";
import { useModelProvidersQuery, modelProviderKeys } from "@/hooks/api/use-model-providers";

type UseModelProvidersAssociationsDataInput = {
  selectedModelId: number | null;
  loadProviderStatus: (providers: ModelWithProvider[], modelId: number) => Promise<void>;
};

export function useModelProvidersAssociationsData({
  selectedModelId,
  loadProviderStatus,
}: UseModelProvidersAssociationsDataInput) {
  const queryClient = useQueryClient();
  const { data: modelProviders = [] } = useModelProvidersQuery(selectedModelId);

  // 列表变空时也要刷新状态，避免删除最后一个关联后 providerStatus 残留
  useEffect(() => {
    if (selectedModelId !== null) {
      void loadProviderStatus(modelProviders, selectedModelId);
    }
  }, [modelProviders, selectedModelId, loadProviderStatus]);

  const fetchModelProviders = useCallback(
    async (modelId: number) => {
      // 只触发 query refetch；状态同步交给上方 effect（query 回流后自动调 loadProviderStatus）
      await queryClient.refetchQueries({ queryKey: modelProviderKeys.list(modelId) });
    },
    [queryClient],
  );

  const setModelProviders = useCallback(
    (updater: ModelWithProvider[] | ((prev: ModelWithProvider[]) => ModelWithProvider[])) => {
      if (selectedModelId === null) return;
      queryClient.setQueryData<ModelWithProvider[]>(
        modelProviderKeys.list(selectedModelId),
        (old) => {
          if (!old) return old;
          return typeof updater === "function" ? updater(old) : updater;
        },
      );
    },
    [queryClient, selectedModelId],
  );

  return { modelProviders, setModelProviders, fetchModelProviders };
}
