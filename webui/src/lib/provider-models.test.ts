import { describe, expect, it } from "vitest";
import type { GroupRef, Provider, ProviderModel, ProviderModelCatalog, UpstreamModelEntry } from "./api";
import {
  buildConfigWithCustomModels,
  buildModelGroupMap,
  buildProviderModelGroups,
  extractUniqueGroups,
  extractUniqueGroupsFromCatalog,
  matchModelGroupFilter,
} from "./provider-models";

const provider = (overrides: Partial<Provider>): Provider => ({
  ID: 1,
  Name: "p1",
  Type: "openai",
  Config: "{}",
  Console: "",
  Proxy: "",
  ...overrides,
});

const up = (id: string, groupNames: string[] = []): UpstreamModelEntry => ({
  ID: id,
  Groups: groupNames.map((name, i) => ({ ID: i + 1, Name: name })),
});

const catalogEntry = (overrides: Partial<ProviderModelCatalog>): ProviderModelCatalog => ({
  ProviderID: 1,
  Upstream: [],
  Custom: [],
  ...overrides,
});

describe("buildProviderModelGroups（聚合目录派生）", () => {
  it("每 provider 取 Upstream+Custom 并集去重并包装 owner 字段", () => {
    const groups = buildProviderModelGroups(
      [provider({ ID: 1, Name: "alpha" }), provider({ ID: 2, Name: "beta" })],
      [
        catalogEntry({ ProviderID: 1, Upstream: [up("u1"), up(" shared ")], Custom: ["c1", "shared"] }),
        catalogEntry({ ProviderID: 2, Upstream: [up("u2")], Custom: [] }),
      ]
    );

    expect(groups.map((g) => g.models.map((m) => m.id))).toEqual([
      ["u1", "shared", "c1"],
      ["u2"],
    ]);
    expect(groups[0].models[0]).toMatchObject({
      object: "cached",
      providerId: 1,
      providerName: "alpha",
    });
  });

  it("catalog 缺失的 provider 得空 models（未同步供应商）", () => {
    const groups = buildProviderModelGroups([provider({ ID: 9 })], [
      catalogEntry({ ProviderID: 1, Upstream: [up("u1")] }),
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0].models).toEqual([]);
  });

  it("catalog 中 providers 列表没有的条目被忽略（以 providers 为主序）", () => {
    const groups = buildProviderModelGroups([provider({ ID: 1 })], [
      catalogEntry({ ProviderID: 1 }),
      catalogEntry({ ProviderID: 99, Upstream: [up("ghost")] }),
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0].provider.ID).toBe(1);
  });

  it("并集保序：Upstream 先于 Custom，重复以首次出现为准", () => {
    const [group] = buildProviderModelGroups(
      [provider({ ID: 1 })],
      [catalogEntry({ ProviderID: 1, Upstream: [up("u1"), up("u2")], Custom: ["u2", "c1"] })]
    );

    expect(group.models.map((m) => m.id)).toEqual(["u1", "u2", "c1"]);
  });

  it("Upstream 模型携带 Groups，Custom 模型无 Groups", () => {
    const [group] = buildProviderModelGroups(
      [provider({ ID: 1 })],
      [
        catalogEntry({
          ProviderID: 1,
          Upstream: [up("u1", ["低价组", "走量组"])],
          Custom: ["c1"],
        }),
      ]
    );

    const u1 = group.models.find((m) => m.id === "u1");
    expect(u1?.Groups).toEqual([
      { ID: 1, Name: "低价组" },
      { ID: 2, Name: "走量组" },
    ]);
    const c1 = group.models.find((m) => m.id === "c1");
    expect(c1?.Groups).toBeUndefined();
  });
});

describe("buildModelGroupMap（分组归属映射）", () => {
  it("从 catalog 条目提取 model → Groups 映射", () => {
    const map = buildModelGroupMap(
      catalogEntry({ Upstream: [up("m1", ["g1"]), up("m2", ["g1", "g2"])] })
    );
    expect(map.get("m1")).toEqual([{ ID: 1, Name: "g1" }]);
    expect(map.get("m2")).toHaveLength(2);
  });

  it("无 Groups 的条目不进 map", () => {
    const map = buildModelGroupMap(catalogEntry({ Upstream: [up("m1")] }));
    expect(map.has("m1")).toBe(false);
  });

  it("undefined entry 返回空 map", () => {
    expect(buildModelGroupMap(undefined).size).toBe(0);
  });
});

describe("buildConfigWithCustomModels（custom-only 写入）", () => {
  it("只重写 custom_models，其余键（含残留 upstream_models 死键）原样保留", () => {
    const config = JSON.stringify({ x: 1, upstream_models: ["legacy"], custom_models: ["old"] });
    const next = buildConfigWithCustomModels(config, [" c1 ", "c1", ""]);
    expect(JSON.parse(next)).toEqual({
      x: 1,
      upstream_models: ["legacy"],
      custom_models: ["c1"],
    });
  });

  it("非法 config 退化为仅含 custom_models 的新对象", () => {
    const next = buildConfigWithCustomModels("not-json", ["c1"]);
    expect(JSON.parse(next)).toEqual({ custom_models: ["c1"] });
  });

  it("空列表清空 custom_models 键", () => {
    const config = JSON.stringify({ custom_models: ["old"], k: "v" });
    const next = buildConfigWithCustomModels(config, []);
    expect(JSON.parse(next)).toEqual({ custom_models: [], k: "v" });
  });
});

describe("matchModelGroupFilter（分组筛选匹配）", () => {
  const g1: GroupRef = { ID: 1, Name: "低价组" };
  const g2: GroupRef = { ID: 2, Name: "走量组" };

  it('"all" 或空串 = 不筛，恒 true', () => {
    expect(matchModelGroupFilter(undefined, "all")).toBe(true);
    expect(matchModelGroupFilter([], "all")).toBe(true);
    expect(matchModelGroupFilter([g1], "")).toBe(true);
  });

  it("指定组 ID = 该组在列表中即 true", () => {
    expect(matchModelGroupFilter([g1, g2], "2")).toBe(true);
    expect(matchModelGroupFilter([g1], "2")).toBe(false);
    expect(matchModelGroupFilter(undefined, "1")).toBe(false);
  });
});

describe("extractUniqueGroups / extractUniqueGroupsFromCatalog（去重分组列表）", () => {
  const g1: GroupRef = { ID: 1, Name: "A" };
  const g2: GroupRef = { ID: 2, Name: "B" };
  const pm = (id: string, groups?: GroupRef[]): ProviderModel => ({
    id,
    object: "cached",
    created: 0,
    owned_by: "local",
    Groups: groups,
  });

  it("从模型列表按首次出现去重", () => {
    const result = extractUniqueGroups([pm("m1", [g1, g2]), pm("m2", [g2]), pm("m3")]);
    expect(result).toEqual([g1, g2]);
  });

  it("从 catalog 条目按首次出现去重", () => {
    const result = extractUniqueGroupsFromCatalog({
      Upstream: [
        { ID: "m1", Groups: [g1, g2] },
        { ID: "m2", Groups: [g2] },
      ],
    });
    expect(result).toEqual([g1, g2]);
  });

  it("undefined catalog 返回空数组", () => {
    expect(extractUniqueGroupsFromCatalog(undefined)).toEqual([]);
  });
});
