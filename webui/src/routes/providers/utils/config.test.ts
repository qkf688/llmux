import { describe, expect, it } from "vitest";
import type { ProviderDetail } from "@/lib/api";
import { buildConfigFromForm, buildProviderPayload, detailToFormValues } from "./config";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";

function formValues(overrides: Partial<ProviderFormValues> = {}): ProviderFormValues {
  return {
    ...defaultProviderFormValues,
    type: "openai",
    base_url: "https://api.example.com/v1",
    protocols: ["openai"],
    endpoints: [{ protocol: "openai", url: "", enabled: true }],
    groups: [{ name: "默认组", weight: 1, poolId: "7" }],
    ...overrides,
  };
}

function detail(overrides: Partial<ProviderDetail> = {}): ProviderDetail {
  return {
    ID: 1,
    Name: "p-detail",
    Type: "openai",
    Config: '{"base_url":"https://api.example.com/v1"}',
    Console: "",
    Proxy: "",
    Protocols: ["openai"],
    Endpoints: [{ ID: 11, ProviderID: 1, Protocol: "openai", URL: "", Enabled: true }],
    Groups: [
      {
        ID: 21,
        ProviderID: 1,
        Name: "默认组",
        Weight: 1,
        Models: "",
        PoolID: 9,
      },
    ],
    ...overrides,
  } as ProviderDetail;
}

describe("buildConfigFromForm - api_key 治理（AC-2）", () => {
  it("config 不含 api_key 键（凭据只走号池）", () => {
    const config = JSON.parse(buildConfigFromForm(formValues()));
    expect(config).not.toHaveProperty("api_key");
  });

  it("config 不含 _schedule 键（协议/端点/分组走独立 DTO）", () => {
    const config = JSON.parse(buildConfigFromForm(formValues()));
    expect(config).not.toHaveProperty("_schedule");
  });

  it("config 保留 adapter 字段与 custom_models", () => {
    const config = JSON.parse(
      buildConfigFromForm(formValues({ custom_models: "m-a\nm-b" }))
    );
    expect(config.base_url).toBe("https://api.example.com/v1");
    expect(config.custom_models).toEqual(["m-a", "m-b"]);
  });
});

describe("buildConfigFromForm - 协议驱动的 adapter 字段", () => {
  it("主类型 openai 但勾选 anthropic：config 写入 anthropic 的 version/beta/auth_type", () => {
    const config = JSON.parse(
      buildConfigFromForm(
        formValues({
          type: "openai",
          protocols: ["openai", "anthropic"],
          version: "2023-06-01",
          beta: "beta-1",
          auth_type: "bearer",
        })
      )
    );
    // anthropic 端点按端点协议实例化 provider，读 config.version/beta/auth_type——
    // 只要勾选 anthropic 协议就必须能配置并写入，与主类型无关
    expect(config.version).toBe("2023-06-01");
    expect(config.beta).toBe("beta-1");
    expect(config.auth_type).toBe("bearer");
  });

  it("未勾选 anthropic：config 不含 anthropic 的 auth_type/version/beta（4b 拆键互不串方言），openai 方言写 openai_auth_type", () => {
    const config = JSON.parse(
      buildConfigFromForm(
        formValues({
          type: "openai",
          protocols: ["openai"],
          version: "2023-06-01",
          beta: "beta-1",
          auth_type: "bearer",
        })
      )
    );
    expect(config).not.toHaveProperty("version");
    expect(config).not.toHaveProperty("beta");
    // auth_type 键归 anthropic 独占（存量混合行里的 x-api-key 不会翻转 openai 端点，P0 回归钉）
    expect(config).not.toHaveProperty("auth_type");
    expect(config.openai_auth_type).toBe("bearer");
  });

  it("主类型 openai、方言未显式选择：提交回退 openai 默认 bearer（不带上 anthropic 的 x-api-key）", () => {
    const config = JSON.parse(
      buildConfigFromForm(
        formValues({ type: "openai", protocols: ["openai"], auth_type: undefined })
      )
    );
    expect(config.openai_auth_type).toBe("bearer");
  });
});

describe("buildProviderPayload - 分组只引用号池", () => {
  it("payload 携带 protocols/endpoints/groups，config 内嵌无 api_key", () => {
    const payload = buildProviderPayload(formValues(), { name: "p", type: "openai" });

    expect(payload.name).toBe("p");
    expect(payload.protocols).toEqual(["openai"]);
    expect(payload.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);

    const config = JSON.parse(payload.config ?? "{}");
    expect(config).not.toHaveProperty("api_key");
    expect(config).not.toHaveProperty("_schedule");
  });

  it("分组恒输出 {id,name,weight,pool_id}，poolId 字符串转数字", () => {
    const payload = buildProviderPayload(formValues());
    expect(payload.groups).toEqual([
      { id: 0, name: "默认组", weight: 1, pool_id: 7 },
    ]);
  });

  it("分组不携带任何 key 字段（内联凭据已删除）：payload 分组键集合恒为四项", () => {
    const payload = buildProviderPayload(formValues());
    for (const group of payload.groups ?? []) {
      expect(Object.keys(group).sort()).toEqual(["id", "name", "pool_id", "weight"]);
    }
  });

  it("组 id 随行回传（就地更新键），保证模型白名单不被保存冲掉", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [{ name: "池组", weight: 2, groupId: 22, poolId: "9" }],
      })
    );
    expect(payload.groups).toEqual([
      { id: 22, name: "池组", weight: 2, pool_id: 9 },
    ]);
  });

  it("poolId 为空串时提交 null（schema 已拦必填，这里是绕过校验时的兜底形状）", () => {
    const payload = buildProviderPayload(
      formValues({ groups: [{ name: "未选池", weight: 1, poolId: "" }] })
    );
    expect(payload.groups).toEqual([
      { id: 0, name: "未选池", weight: 1, pool_id: null },
    ]);
  });
});

describe("detailToFormValues - 详情回填", () => {
  it("分组只回填 {name,weight,groupId,poolId}，不携带任何凭据行", () => {
    const values = detailToFormValues(detail());

    expect(values.name).toBe("p-detail");
    expect(values.base_url).toBe("https://api.example.com/v1");
    expect(values.protocols).toEqual(["openai"]);
    expect(values.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);
    expect(values.groups).toEqual([
      { name: "默认组", weight: 1, groupId: 21, poolId: "9" },
    ]);
    // 顶层 api_key 已废除：表单值不得再出现该键（防止回写明文进 config）
    expect("api_key" in values).toBe(false);
  });

  it("存量脏数据 PoolID 为 null：回填空串（前端校验要求用户重选号池）", () => {
    const values = detailToFormValues(
      detail({
        Groups: [
          { ID: 23, ProviderID: 1, Name: "无池组", Weight: 1, Models: "", PoolID: null },
        ],
      })
    );
    expect(values.groups?.[0]).toEqual({
      name: "无池组",
      weight: 1,
      groupId: 23,
      poolId: "",
    });
  });

  it("anthropic 型：version/beta/auth_type 从 config 解析回填", () => {
    const values = detailToFormValues(
      detail({
        Type: "anthropic",
        Protocols: ["anthropic"],
        Endpoints: [{ ID: 12, ProviderID: 1, Protocol: "anthropic", URL: "https://x/claude/v1", Enabled: true }],
        Config: '{"base_url":"https://x","version":"2023-06-01","beta":"b1","auth_type":"bearer"}',
      })
    );

    expect(values.type).toBe("anthropic");
    expect(values.version).toBe("2023-06-01");
    expect(values.beta).toBe("b1");
    expect(values.auth_type).toBe("bearer");
    expect(values.endpoints).toEqual([
      { protocol: "anthropic", url: "https://x/claude/v1", enabled: true },
    ]);
  });

  it("openai 型 config 缺方言键（存量行）：回填 openai_auth_type=bearer，anthropic 键保持空（4b 拆键）", () => {
    const values = detailToFormValues(
      detail({ Type: "openai", Protocols: ["openai"], Config: '{"base_url":"https://x"}' }),
    );
    expect(values.openai_auth_type).toBe("bearer");
    expect(values.auth_type).toBe("");
  });

  it("anthropic 型 config 缺 auth_type：回填类型默认 x-api-key", () => {
    const values = detailToFormValues(
      detail({ Type: "anthropic", Protocols: ["anthropic"], Config: '{"base_url":"https://x","version":"2023-06-01"}' }),
    );
    expect(values.auth_type).toBe("x-api-key");
  });

  it("Protocols 为空时回退空数组（编辑侧不做 type 派生，类型勾选交给用户）", () => {
    const values = detailToFormValues(detail({ Protocols: [] }));
    expect(values.protocols).toEqual([]);
  });
});