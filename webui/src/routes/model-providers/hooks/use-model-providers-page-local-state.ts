import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import type { Model, Provider, Settings } from "@/lib/api";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

export function useModelProvidersPageLocalState() {
  const [models, setModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();

  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);

  return {
    models,
    setModels,
    providers,
    setProviders,
    searchParams,
    setSearchParams,
    selectedModelId,
    setSelectedModelId,
    statusUpdating,
    setStatusUpdating,
    statusError,
    setStatusError,
    providerModelGroups,
    setProviderModelGroups,
    providerModels,
    setProviderModels,
    settings,
    setSettings,
  };
}

