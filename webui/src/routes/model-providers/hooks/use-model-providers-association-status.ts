import { useCallback, useState } from "react";
import { getModelProviderHealthStatus, getModelProviderStatus, type Model, type ModelWithProvider } from "@/lib/api";

type ProviderStatusState = Record<number, boolean[]>;

export function useModelProvidersAssociationStatus(models: Model[]) {
  const [providerStatus, setProviderStatus] = useState<ProviderStatusState>({});
  const [healthStatus, setHealthStatus] = useState<ProviderStatusState>({});

  const loadProviderStatus = useCallback(
    async (providers: ModelWithProvider[], modelId: number) => {
      const selectedModel = models.find((model) => model.ID === modelId);
      // 即使找不到 model 也先 reset，避免上一个 model 的 providerStatus 残留
      setProviderStatus({});
      setHealthStatus({});
      if (!selectedModel) return;

      const newStatus: ProviderStatusState = {};
      const newHealthStatus: ProviderStatusState = {};

      await Promise.all(
        providers.map(async (provider) => {
          try {
            const [status, healthStatusList] = await Promise.all([
              getModelProviderStatus(provider.ProviderID, selectedModel.Name, provider.ProviderModel),
              getModelProviderHealthStatus(provider.ID),
            ]);
            newStatus[provider.ID] = status;
            newHealthStatus[provider.ID] = healthStatusList;
          } catch (error) {
            console.error(`Failed to load status for provider ${provider.ID}:`, error);
            newStatus[provider.ID] = [];
            newHealthStatus[provider.ID] = [];
          }
        })
      );

      setProviderStatus(newStatus);
      setHealthStatus(newHealthStatus);
    },
    [models]
  );

  return { providerStatus, healthStatus, loadProviderStatus };
}

