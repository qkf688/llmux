import { useEffect } from "react";
import type { Model } from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import type { FormValues } from "../form-schema";

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersBootstrapInput = {
  models: Model[];
  resetTransient: () => void;
  selectedModelId: number | null;
  setSelectedModelId: Setter<number | null>;
  searchParams: URLSearchParams;
  setSearchParams: (next: URLSearchParams, options?: { replace?: boolean }) => void;
  setFormModelId: (value: FormValues["model_id"]) => void;
};

export function useModelProvidersBootstrap({
  models,
  resetTransient,
  selectedModelId,
  setSelectedModelId,
  searchParams,
  setSearchParams,
  setFormModelId,
}: UseModelProvidersBootstrapInput) {
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
}
