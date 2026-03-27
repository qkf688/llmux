import { useCallback } from "react";
import type { ProviderModelSelection, ProviderModelWithOwner } from "../types";
import { buildSelectionKey } from "../utils/selection";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersModelListSelectionInput = {
  setModelSearchKeyword: (keyword: string) => void;
  setModelListDialogOpen: (open: boolean) => void;
  setSelectedProviderModels: Setter<ProviderModelSelection[]>;
  visibleAvailableModels: ProviderModelWithOwner[];
};

export function useModelProvidersModelListSelection({
  setModelSearchKeyword,
  setModelListDialogOpen,
  setSelectedProviderModels,
  visibleAvailableModels,
}: UseModelProvidersModelListSelectionInput) {
  const openModelListDialog = useCallback(() => {
    setModelSearchKeyword("");
    setModelListDialogOpen(true);
  }, [setModelListDialogOpen, setModelSearchKeyword]);

  const clearSelectedProviderModels = useCallback(() => {
    setSelectedProviderModels([]);
  }, [setSelectedProviderModels]);

  const removeSelectedProviderModel = useCallback(
    (selectionKey: string) => {
      setSelectedProviderModels((prev) =>
        prev.filter((item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey)
      );
    },
    [setSelectedProviderModels]
  );

  const handleProviderChange = useCallback(() => {
    setSelectedProviderModels([]);
  }, [setSelectedProviderModels]);

  const selectAllVisibleAvailable = useCallback(() => {
    setSelectedProviderModels((prev) => {
      const merged = new Map(prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item]));
      visibleAvailableModels.forEach((model) => {
        merged.set(buildSelectionKey(model.providerId, model.id), {
          providerId: model.providerId,
          providerName: model.providerName,
          modelId: model.id,
        });
      });
      return Array.from(merged.values());
    });
  }, [setSelectedProviderModels, visibleAvailableModels]);

  const clearModelListSelection = useCallback(() => {
    setSelectedProviderModels([]);
  }, [setSelectedProviderModels]);

  const toggleModelSelection = useCallback(
    (model: ProviderModelWithOwner, checkedValue: boolean, selectionKey: string) => {
      if (checkedValue) {
        setSelectedProviderModels((prev) => {
          const merged = new Map(prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item]));
          merged.set(selectionKey, {
            providerId: model.providerId,
            providerName: model.providerName,
            modelId: model.id,
          });
          return Array.from(merged.values());
        });
        return;
      }

      setSelectedProviderModels((prev) =>
        prev.filter((item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey)
      );
    },
    [setSelectedProviderModels]
  );

  return {
    openModelListDialog,
    clearSelectedProviderModels,
    removeSelectedProviderModel,
    handleProviderChange,
    selectAllVisibleAvailable,
    clearModelListSelection,
    toggleModelSelection,
  };
}

