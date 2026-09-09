import type { GroupRef, Provider, ProviderModel, ProviderModelCatalog } from "./api";

// 模型目录数据源 = 聚合端点 GET /providers/model-catalog（分组白名单并集 +
// custom_models）。本文件只做派生与写序列化，不再解析 Config.upstream_models
// （S5 后该键为死键，上游真相在供应商分组白名单）。

const normalize = (name: string) => name.trim();

export function toProviderModelList(models: string[]): ProviderModel[] {
  const now = Date.now();
  return models.map((model) => ({
    id: model,
    object: "cached",
    created: Math.floor(now / 1000),
    owned_by: "local",
  }));
}

/** 目录写路径唯一合法入口：只重写 custom_models 键，其余 config 键（含 S5 后
 *  残留的 upstream_models 死键）不读不写原样保留。上游来源的编辑入口在供应商
 *  表单分组白名单，不走本函数。 */
export function buildConfigWithCustomModels(config: string, customModels: string[]): string {
  const uniqueCustom = Array.from(new Set(customModels.map(normalize).filter(Boolean)));
  try {
    const parsed = JSON.parse(config || "{}");
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return JSON.stringify({ ...parsed, custom_models: uniqueCustom });
    }
  } catch {
    // ignore, fallback below
  }
  return JSON.stringify({ custom_models: uniqueCustom });
}

/** 模型条目 + 所属供应商标注（目录展示用） */
export type ProviderModelWithOwner = ProviderModel & {
  providerId: number;
  providerName: string;
};

/** 供应商分组目录：provider + 该供应商的全量模型（Upstream+Custom 并集） */
export type ProviderModelGroup = {
  provider: Provider;
  models: ProviderModelWithOwner[];
};

/** 单个目录条目的 Upstream+Custom 并集（trim/去重/保序，Upstream 先于 Custom）。
 *  同时出现在两个来源的模型只保留一次。 */
export function unionCatalogModels(entry: Pick<ProviderModelCatalog, "Upstream" | "Custom">): string[] {
  const seen = new Set<string>();
  for (const item of entry.Upstream) {
    const name = normalize(item.ID);
    if (name) {
      seen.add(name);
    }
  }
  for (const model of entry.Custom) {
    const name = normalize(model);
    if (name) {
      seen.add(name);
    }
  }
  return [...seen];
}

/** 从 catalog 条目提取 model ID → 分组列表映射（徽标与筛选用）。
 *  Custom 模型无分组归属，不在 map 中。 */
export function buildModelGroupMap(
  entry: Pick<ProviderModelCatalog, "Upstream"> | undefined
): Map<string, GroupRef[]> {
  const map = new Map<string, GroupRef[]>();
  if (!entry) return map;
  for (const item of entry.Upstream) {
    const name = normalize(item.ID);
    if (name && item.Groups.length > 0) {
      map.set(name, item.Groups);
    }
  }
  return map;
}

/** 从 ListUpstreamModels 响应提取 model ID → 分组列表映射。 */
export function buildUpstreamModelGroupMap(models: ProviderModel[]): Map<string, GroupRef[]> {
  const map = new Map<string, GroupRef[]>();
  for (const m of models) {
    if (m.Groups && m.Groups.length > 0) {
      map.set(m.id, m.Groups);
    }
  }
  return map;
}

/** 分组筛选值约定：`"all"`=不筛、`"__solo__"`=仅单分组可服务、其余=分组 ID 字符串。 */
export const GROUP_FILTER_SOLO = "__solo__";

/** 判断模型的分组列表是否匹配当前筛选值。 */
export function matchModelGroupFilter(
  groups: GroupRef[] | undefined,
  filter: string
): boolean {
  if (!filter || filter === "all") return true;
  if (filter === GROUP_FILTER_SOLO) return !!groups && groups.length === 1;
  return !!groups && groups.some((g) => String(g.ID) === filter);
}

/** 从模型列表提取去重后的分组列表（按首次出现顺序）。 */
export function extractUniqueGroups(models: ProviderModel[]): GroupRef[] {
  const seen = new Set<number>();
  const out: GroupRef[] = [];
  for (const m of models) {
    for (const g of m.Groups ?? []) {
      if (!seen.has(g.ID)) {
        seen.add(g.ID);
        out.push(g);
      }
    }
  }
  return out;
}

/** 从 catalog 条目提取去重后的分组列表（按首次出现顺序）。 */
export function extractUniqueGroupsFromCatalog(
  entry: Pick<ProviderModelCatalog, "Upstream"> | undefined
): GroupRef[] {
  if (!entry) return [];
  const seen = new Set<number>();
  const out: GroupRef[] = [];
  for (const item of entry.Upstream) {
    for (const g of item.Groups) {
      if (!seen.has(g.ID)) {
        seen.add(g.ID);
        out.push(g);
      }
    }
  }
  return out;
}

/** 聚合目录 → 供应商分组模型目录（models/model-providers 页共享构建逻辑）。
 *  以 providers 列表为主序：catalog 缺失的 provider 得空 models（未同步），
 *  catalog 多余条目被忽略。 */
export function buildProviderModelGroups(
  providers: Provider[],
  catalog: ProviderModelCatalog[]
): ProviderModelGroup[] {
  const catalogByProvider = new Map(catalog.map((entry) => [entry.ProviderID, entry]));
  return providers.map((provider) => {
    const entry = catalogByProvider.get(provider.ID);
    const groupMap = buildModelGroupMap(entry);
    const models: ProviderModelWithOwner[] = toProviderModelList(
      unionCatalogModels(entry ?? { Upstream: [], Custom: [] })
    ).map((model) => ({
      ...model,
      providerId: provider.ID,
      providerName: provider.Name,
      Groups: groupMap.get(model.id),
    }));
    return { provider, models };
  });
}
