import type { ProviderModelGroup } from "@/lib/provider-models";

/** 构建逻辑已下沉 lib/provider-models.ts（buildProviderModelGroups，
 *  数据源为聚合目录 API），本文件保留 models 页专属的折叠/筛选纯函数。 */

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
    // 无关键词时保留空目录分组（未同步供应商可见，引导先同步）；有关键词时空分组无匹配自然消失
    .filter((group) => group.models.length > 0 || normalizedKeyword === "");
};
