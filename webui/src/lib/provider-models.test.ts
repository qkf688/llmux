import { describe, expect, it } from "vitest";
import type { Provider, ProviderModelCatalog } from "./api";
import {
  buildConfigWithCustomModels,
  buildProviderModelGroups,
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
        catalogEntry({ ProviderID: 1, Upstream: ["u1", " shared "], Custom: ["c1", "shared"] }),
        catalogEntry({ ProviderID: 2, Upstream: ["u2"], Custom: [] }),
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
      catalogEntry({ ProviderID: 1, Upstream: ["u1"] }),
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0].models).toEqual([]);
  });

  it("catalog 中 providers 列表没有的条目被忽略（以 providers 为主序）", () => {
    const groups = buildProviderModelGroups([provider({ ID: 1 })], [
      catalogEntry({ ProviderID: 1 }),
      catalogEntry({ ProviderID: 99, Upstream: ["ghost"] }),
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0].provider.ID).toBe(1);
  });

  it("并集保序：Upstream 先于 Custom，重复以首次出现为准", () => {
    const [group] = buildProviderModelGroups(
      [provider({ ID: 1 })],
      [catalogEntry({ ProviderID: 1, Upstream: ["u1", "u2"], Custom: ["u2", "c1"] })]
    );

    expect(group.models.map((m) => m.id)).toEqual(["u1", "u2", "c1"]);
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
