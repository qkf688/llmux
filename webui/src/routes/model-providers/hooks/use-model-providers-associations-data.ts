import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { getModelProviders, type ModelWithProvider } from "@/lib/api";

type UseModelProvidersAssociationsDataInput = {
  selectedModelId: number | null;
  setLoading: (loading: boolean) => void;
  loadProviderStatus: (providers: ModelWithProvider[], modelId: number) => Promise<void>;
};

export function useModelProvidersAssociationsData({
  selectedModelId,
  setLoading,
  loadProviderStatus,
}: UseModelProvidersAssociationsDataInput) {
  const [modelProviders, setModelProviders] = useState<ModelWithProvider[]>([]);

  const fetchModelProviders = useCallback(
    async (modelId: number) => {
      try {
        setLoading(true);
        const data = await getModelProviders(modelId);
        setModelProviders(
          data.map((item) => ({
            ...item,
            CustomerHeaders: item.CustomerHeaders || {},
          }))
        );
        void loadProviderStatus(data, modelId);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`获取模型提供商关联列表失败: ${message}`);
        console.error(err);
      } finally {
        setLoading(false);
      }
    },
    [loadProviderStatus, setLoading]
  );

  useEffect(() => {
    if (selectedModelId) {
      void fetchModelProviders(selectedModelId);
    }
  }, [fetchModelProviders, selectedModelId]);

  return { modelProviders, setModelProviders, fetchModelProviders };
}

