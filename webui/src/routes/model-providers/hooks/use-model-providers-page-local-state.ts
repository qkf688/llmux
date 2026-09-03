import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useModels } from "@/hooks/api/use-models";
import { useProviderModelCatalog, useProviders, useSettings } from "@/hooks/api/use-providers";
import { EMPTY_MODELS, EMPTY_MODEL_CATALOG, EMPTY_PROVIDERS } from "@/lib/empty-constants";
import { buildProviderModelGroups } from "@/lib/provider-models";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

export function useModelProvidersPageLocalState() {
  const { data: models = EMPTY_MODELS, isLoading: modelsLoading } = useModels();
  const { data: providers = EMPTY_PROVIDERS, isLoading: providersLoading } = useProviders();
  const { data: catalogData = EMPTY_MODEL_CATALOG, isLoading: catalogLoading } =
    useProviderModelCatalog();
  const { data: settingsData } = useSettings();
  const settings = settingsData ?? null;

  const searchParams = useSearchParams();
  const [searchParamsValue, setSearchParamsValue] = searchParams;

  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);

  // 目录数据源：聚合 API（与 models 页共享构建逻辑，见 lib/provider-models.ts）
  const providerModelGroups = useMemo<ProviderModelGroup[]>(
    () => buildProviderModelGroups(providers, catalogData),
    [providers, catalogData],
  );

  const providerModels = useMemo<ProviderModelWithOwner[]>(
    () => providerModelGroups.flatMap((group) => group.models),
    [providerModelGroups],
  );

  const loading = modelsLoading || providersLoading || catalogLoading;

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
