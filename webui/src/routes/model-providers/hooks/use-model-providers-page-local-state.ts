import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useModels } from "@/hooks/api/use-models";
import { useProviders } from "@/hooks/api/use-providers";
import { useSettings } from "@/hooks/api/use-providers";
import { EMPTY_MODELS, EMPTY_PROVIDERS } from "@/lib/empty-constants";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

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
