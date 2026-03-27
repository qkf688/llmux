import { useCallback, useEffect } from "react";
import { toast } from "sonner";
import { getModels, getProviders, getSettings, type Model, type Provider, type Settings } from "@/lib/api";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";
import type { FormValues } from "../form-schema";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersBootstrapInput = {
  models: Model[];
  setModels: Setter<Model[]>;
  setProviders: Setter<Provider[]>;
  setSettings: (settings: Settings | null) => void;

  setLoading: (loading: boolean) => void;
  setLoadingProviderModels: (loading: boolean) => void;
  setProviderModelGroups: Setter<ProviderModelGroup[]>;
  setProviderModels: Setter<ProviderModelWithOwner[]>;
  setCollapsedProviders: Setter<Record<number, boolean>>;
  resetTransient: () => void;

  selectedModelId: number | null;
  setSelectedModelId: Setter<number | null>;

  searchParams: URLSearchParams;
  setSearchParams: (next: URLSearchParams, options?: { replace?: boolean }) => void;

  setFormModelId: (value: FormValues["model_id"]) => void;
};

export function useModelProvidersBootstrap({
  models,
  setModels,
  setProviders,
  setSettings,
  setLoading,
  setLoadingProviderModels,
  setProviderModelGroups,
  setProviderModels,
  setCollapsedProviders,
  resetTransient,
  selectedModelId,
  setSelectedModelId,
  searchParams,
  setSearchParams,
  setFormModelId,
}: UseModelProvidersBootstrapInput) {
  const rebuildProviderModels = useCallback(
    (providerList: Provider[]) => {
      setLoadingProviderModels(true);

      const groups = providerList.map((provider) => {
        const models = toProviderModelList(parseAllModelsFromConfig(provider.Config)).map((model) => ({
          ...model,
          providerId: provider.ID,
          providerName: provider.Name,
        }));
        return { provider, models };
      });

      setProviderModelGroups(groups);
      setProviderModels(groups.flatMap((group) => group.models));
      setCollapsedProviders((prev) => {
        const next: Record<number, boolean> = {};
        groups.forEach(({ provider }) => {
          next[provider.ID] = prev[provider.ID] ?? false;
        });
        return next;
      });

      setLoadingProviderModels(false);
    },
    [setCollapsedProviders, setLoadingProviderModels, setProviderModelGroups, setProviderModels]
  );

  const fetchModels = useCallback(async () => {
    try {
      const data = await getModels();
      setModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型列表失败: ${message}`);
      console.error(err);
    }
  }, [setModels]);

  const fetchProviders = useCallback(async () => {
    try {
      const data = await getProviders();
      setProviders(data);
      rebuildProviderModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商列表失败: ${message}`);
      console.error(err);
    }
  }, [rebuildProviderModels, setProviders]);

  const fetchSettings = useCallback(async () => {
    try {
      const data = await getSettings();
      setSettings(data);
    } catch (err) {
      console.error("获取系统设置失败", err);
    }
  }, [setSettings]);

  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  useEffect(() => {
    if (models.length === 0) {
      if (selectedModelId !== null) {
        setSelectedModelId(null);
        setFormModelId(0);
      }
      return;
    }

    const modelIdParam = searchParams.get("modelId");
    const parsedParam = modelIdParam ? Number(modelIdParam) : Number.NaN;

    if (!Number.isNaN(parsedParam) && models.some((model) => model.ID === parsedParam)) {
      if (selectedModelId !== parsedParam) {
        setSelectedModelId(parsedParam);
        setFormModelId(parsedParam);
      }
      return;
    }

    const fallbackId = models[0].ID;
    if (selectedModelId !== fallbackId) {
      setSelectedModelId(fallbackId);
      setFormModelId(fallbackId);
    }
    if (modelIdParam !== fallbackId.toString()) {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set("modelId", fallbackId.toString());
      setSearchParams(nextParams, { replace: true });
    }
  }, [models, searchParams, selectedModelId, setFormModelId, setSearchParams, setSelectedModelId]);

  useEffect(() => {
    Promise.all([fetchModels(), fetchProviders(), fetchSettings()]).finally(() => {
      setLoading(false);
    });
  }, [fetchModels, fetchProviders, fetchSettings, setLoading]);

  return { fetchModels, fetchProviders, fetchSettings, rebuildProviderModels };
}

