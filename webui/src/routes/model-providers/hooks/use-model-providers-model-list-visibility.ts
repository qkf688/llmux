import { useMemo } from "react";
import { useWatch, type Control } from "react-hook-form";
import type { ModelWithProvider } from "@/lib/api";
import type { FormValues } from "../form-schema";
import type { ProviderModelGroup, ProviderModelSelection } from "../types";
import { buildSelectionKey } from "../utils/selection";

type UseModelProvidersModelListVisibilityInput = {
  control: Control<FormValues>;
  providerModelGroups: ProviderModelGroup[];
  modelSearchKeyword: string;
  modelProviders: ModelWithProvider[];
  selectedProviderModels: ProviderModelSelection[];
};

export function useModelProvidersModelListVisibility({
  control,
  providerModelGroups,
  modelSearchKeyword,
  modelProviders,
  selectedProviderModels,
}: UseModelProvidersModelListVisibilityInput) {
  const selectedProviderId = useWatch({
    control,
    name: "provider_id",
  });

  const existingAssociationKeys = useMemo(() => {
    return new Set(modelProviders.map((mp) => buildSelectionKey(mp.ProviderID, mp.ProviderModel)));
  }, [modelProviders]);

  const visibleProviderGroups = useMemo(() => {
    const searchKeywordLower = modelSearchKeyword.trim().toLowerCase();
    return providerModelGroups
      .filter((group) => (selectedProviderId && selectedProviderId > 0 ? group.provider.ID === selectedProviderId : true))
      .map((group) => ({
        ...group,
        models: group.models.filter((model) => model.id.toLowerCase().includes(searchKeywordLower)),
      }))
      // 无关键词时保留空目录分组（未同步供应商可见，引导先同步）；有关键词时空分组无匹配自然消失
      .filter((group) => group.models.length > 0 || searchKeywordLower === "");
  }, [modelSearchKeyword, providerModelGroups, selectedProviderId]);

  const visibleProviderModels = useMemo(() => {
    return visibleProviderGroups.flatMap((group) => group.models);
  }, [visibleProviderGroups]);

  const visibleAvailableModels = useMemo(() => {
    return visibleProviderModels.filter(
      (model) => !existingAssociationKeys.has(buildSelectionKey(model.providerId, model.id))
    );
  }, [existingAssociationKeys, visibleProviderModels]);

  const visibleExistingCount = useMemo(() => {
    return visibleProviderModels.length - visibleAvailableModels.length;
  }, [visibleAvailableModels.length, visibleProviderModels.length]);

  const selectedKeys = useMemo(() => {
    return new Set(selectedProviderModels.map((item) => buildSelectionKey(item.providerId, item.modelId)));
  }, [selectedProviderModels]);

  return {
    existingAssociationKeys,
    visibleProviderGroups,
    visibleAvailableModels,
    visibleExistingCount,
    selectedKeys,
  };
}
