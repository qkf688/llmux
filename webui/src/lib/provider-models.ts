import type { Provider, ProviderModel, ProviderModelCatalog } from "./api";

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
  for (const model of [...entry.Upstream, ...entry.Custom]) {
    const name = typeof model === "string" ? normalize(model) : "";
    if (name) {
      seen.add(name);
    }
  }
  return [...seen];
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
    const models: ProviderModelWithOwner[] = toProviderModelList(
      unionCatalogModels(entry ?? { Upstream: [], Custom: [] })
    ).map((model) => ({
      ...model,
      providerId: provider.ID,
      providerName: provider.Name,
    }));
    return { provider, models };
  });
}
