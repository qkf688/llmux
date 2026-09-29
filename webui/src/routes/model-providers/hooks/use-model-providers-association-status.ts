import { useCallback, useState } from "react";
import {
  getModelProviderHealthStatus,
  getModelProviderStatus,
  type HealthCheckStatus,
  type Model,
  type ModelWithProvider,
} from "@/lib/api";

/** 最近 10 次请求成败（/model-providers/status，仍是两态布尔） */
type ProviderStatusState = Record<number, boolean[]>;
/** 最近 10 次健康检测（/model-providers/health-status，三态字符串） */
type HealthStatusState = Record<number, HealthCheckStatus[]>;

export function useModelProvidersAssociationStatus(models: Model[]) {
  const [providerStatus, setProviderStatus] = useState<ProviderStatusState>({});
  const [healthStatus, setHealthStatus] = useState<HealthStatusState>({});

  const loadProviderStatus = useCallback(
    async (providers: ModelWithProvider[], modelId: number) => {
      const selectedModel = models.find((model) => model.ID === modelId);
      // 即使找不到 model 或 providers 为空也先 reset，避免上一个 model 的 providerStatus 残留；
      // 提前 return 避免 Promise.all([]) 后再触发一次空对象 setState，消除无意义渲染
      setProviderStatus({});
      setHealthStatus({});
      if (!selectedModel || providers.length === 0) return;

      const newStatus: ProviderStatusState = {};
      const newHealthStatus: HealthStatusState = {};

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

