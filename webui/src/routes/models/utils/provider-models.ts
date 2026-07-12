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
  const nextIds = new Set<number>();

  groups.forEach(({ provider }) => {
    nextIds.add(provider.ID);
    next[provider.ID] = previous[provider.ID] ?? false;
  });

  const previousIds = Object.keys(previous);
  if (
    previousIds.length === nextIds.size &&
    previousIds.every((id) => {
      const providerId = Number(id);
      return nextIds.has(providerId) && previous[providerId] === next[providerId];
    })
  ) {
    return previous;
  }

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
