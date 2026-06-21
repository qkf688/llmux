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

  useEffect(() => {
    if (modelProviders.length > 0 && selectedModelId !== null) {
      void loadProviderStatus(modelProviders, selectedModelId);
    }
  }, [modelProviders, selectedModelId, loadProviderStatus]);

  const fetchModelProviders = useCallback(
    async (modelId: number) => {
      await queryClient.invalidateQueries({ queryKey: modelProviderKeys.list(modelId) });
      void loadProviderStatus(modelProviders, modelId);
    },
    [queryClient, loadProviderStatus, modelProviders],
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
