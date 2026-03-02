import type { Provider } from "@/lib/api";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";

export const buildProviderModelGroups = (providers: Provider[]): ProviderModelGroup[] =>
  providers.map((provider) => {
    const models: ProviderModelWithOwner[] = toProviderModelList(
      parseAllModelsFromConfig(provider.Config)
    ).map((model) => ({
      ...model,
      providerId: provider.ID,
      providerName: provider.Name,
    }));

    return { provider, models };
  });

export const buildCollapsedProviderState = (
  groups: ProviderModelGroup[],
  previous: Record<number, boolean>
): Record<number, boolean> => {
  const next: Record<number, boolean> = {};

  groups.forEach(({ provider }) => {
    next[provider.ID] = previous[provider.ID] ?? false;
  });

  return next;
};

export const filterProviderGroups = (
  groups: ProviderModelGroup[],
  selectedProviderId: string,
  keyword: string
): ProviderModelGroup[] => {
  const normalizedKeyword = keyword.trim().toLowerCase();

  return groups
    .filter(
      (group) => selectedProviderId === "all" || group.provider.ID.toString() === selectedProviderId
    )
    .map((group) => ({
      ...group,
      models: group.models.filter((model) => model.id.toLowerCase().includes(normalizedKeyword)),
    }))
    .filter((group) => group.models.length > 0);
};
