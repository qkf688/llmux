import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useModels } from "@/hooks/api/use-models";
import { useProviders } from "@/hooks/api/use-providers";
import { useSettings } from "@/hooks/api/use-providers";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";
import type { Model, Provider } from "@/lib/api";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

/**
 * 稳定空数组，避免 `data = []` 在 loading 时每次 render 产生新引用触发 effect 循环。
 * 模式复用自 `routes/models/hooks/use-models-page-data.ts`；若第三个页面出现同款循环，
 * 应把 EMPTY_<TYPE> 抽到 `webui/src/lib/` 共享导出，届时本项目所有本地副本一并替换。
 */
const EMPTY_MODELS: Model[] = [];
const EMPTY_PROVIDERS: Provider[] = [];

export function useModelProvidersPageLocalState() {
  const { data: models = EMPTY_MODELS, isLoading: modelsLoading } = useModels();
  const { data: providers = EMPTY_PROVIDERS, isLoading: providersLoading } = useProviders();
  const { data: settingsData } = useSettings();
  const settings = settingsData ?? null;

  const searchParams = useSearchParams();
  const [searchParamsValue, setSearchParamsValue] = searchParams;

  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);

  const providerModelGroups = useMemo<ProviderModelGroup[]>(
    () =>
      providers.map((provider) => {
        const models = toProviderModelList(parseAllModelsFromConfig(provider.Config)).map((model) => ({
          ...model,
          providerId: provider.ID,
          providerName: provider.Name,
        }));
        return { provider, models };
      }),
    [providers],
  );

  const providerModels = useMemo<ProviderModelWithOwner[]>(
    () => providerModelGroups.flatMap((group) => group.models),
    [providerModelGroups],
  );

  const loading = modelsLoading || providersLoading;

  return {
    models,
    providers,
    settings,
    loading,
    searchParams: searchParamsValue,
    setSearchParams: setSearchParamsValue,
    selectedModelId,
    setSelectedModelId,
    statusUpdating,
    setStatusUpdating,
    statusError,
    setStatusError,
    providerModelGroups,
    providerModels,
  };
}
