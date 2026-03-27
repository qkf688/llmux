import { useMemo } from "react";
import type { ModelWithProvider, Provider } from "@/lib/api";

type UseModelProvidersAssociationFiltersInput = {
  modelProviders: ModelWithProvider[];
  providers: Provider[];
  selectedProviderType: string;
  selectedProviderFilter: string;
  selectedStatusFilter: string;
  searchKeyword: string;
  selectedAssociationIds: number[];
};

export function useModelProvidersAssociationFilters({
  modelProviders,
  providers,
  selectedProviderType,
  selectedProviderFilter,
  selectedStatusFilter,
  searchKeyword,
  selectedAssociationIds,
}: UseModelProvidersAssociationFiltersInput) {
  const providerTypes = useMemo(
    () => Array.from(new Set(providers.map((p) => p.Type).filter(Boolean))),
    [providers]
  );

  const filteredModelProviders = useMemo(() => {
    return modelProviders.filter((association) => {
      const provider = providers.find((p) => p.ID === association.ProviderID);

      // 提供商类型筛选
      const typeMatch = selectedProviderType === "all" || provider?.Type === selectedProviderType;

      // 具体提供商筛选
      const providerMatch =
        selectedProviderFilter === "all" || association.ProviderID.toString() === selectedProviderFilter;

      // 启用状态筛选
      const statusMatch =
        selectedStatusFilter === "all" ||
        (selectedStatusFilter === "enabled" && (association.Status ?? true)) ||
        (selectedStatusFilter === "disabled" && !(association.Status ?? true));

      // 搜索关键词筛选
      const keyword = searchKeyword.toLowerCase().trim();
      const searchMatch =
        !keyword ||
        association.ProviderModel.toLowerCase().includes(keyword) ||
        (provider?.Name ?? "").toLowerCase().includes(keyword) ||
        (provider?.Type ?? "").toLowerCase().includes(keyword) ||
        association.ID.toString().includes(keyword);

      return typeMatch && providerMatch && statusMatch && searchMatch;
    });
  }, [modelProviders, providers, searchKeyword, selectedProviderFilter, selectedProviderType, selectedStatusFilter]);

  const hasAssociationFilter = useMemo(() => {
    return (
      selectedProviderType !== "all" ||
      selectedProviderFilter !== "all" ||
      selectedStatusFilter !== "all" ||
      searchKeyword.trim() !== ""
    );
  }, [searchKeyword, selectedProviderFilter, selectedProviderType, selectedStatusFilter]);

  const activeFilterCount = useMemo(() => {
    return [
      selectedProviderType !== "all",
      selectedProviderFilter !== "all",
      selectedStatusFilter !== "all",
      searchKeyword.trim() !== "",
    ].filter(Boolean).length;
  }, [searchKeyword, selectedProviderFilter, selectedProviderType, selectedStatusFilter]);

  const isAllAssociationsSelected = useMemo(() => {
    return filteredModelProviders.length > 0 && selectedAssociationIds.length === filteredModelProviders.length;
  }, [filteredModelProviders.length, selectedAssociationIds.length]);

  const isPartialAssociationsSelected = useMemo(() => {
    return selectedAssociationIds.length > 0 && selectedAssociationIds.length < filteredModelProviders.length;
  }, [filteredModelProviders.length, selectedAssociationIds.length]);

  return {
    providerTypes,
    filteredModelProviders,
    hasAssociationFilter,
    activeFilterCount,
    isAllAssociationsSelected,
    isPartialAssociationsSelected,
  };
}

