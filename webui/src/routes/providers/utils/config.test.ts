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
    groups: [{ name: "默认组", weight: 1, models: "", source: "inline", inlineKeys: "sk-a\nsk-b", poolId: "" }],
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
        Source: "inline",
        InlineKeys: ["sk-plain-1", "sk-plain-2"],
        PoolID: null,
      },
    ],
    ...overrides,
  } as ProviderDetail;
}

describe("buildConfigFromForm - api_key 治理（AC-2）", () => {
  it("config 不含 api_key 键（凭据只走分组内联/号池）", () => {
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

describe("buildProviderPayload - 结构化 DTO（AC-2）", () => {
  it("payload 携带 protocols/endpoints/groups，config 内嵌无 api_key", () => {
    const payload = buildProviderPayload(formValues(), { name: "p", type: "openai" });

    expect(payload.name).toBe("p");
    expect(payload.protocols).toEqual(["openai"]);
    expect(payload.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);

    const config = JSON.parse(payload.config ?? "{}");
    expect(config).not.toHaveProperty("api_key");
    expect(config).not.toHaveProperty("_schedule");
  });

  it("inline 组：inlineKeys 文本拆数组、过滤空行，pool_id 为 null", () => {
    const payload = buildProviderPayload(formValues());
    expect(payload.groups).toEqual([
      {
        name: "默认组",
        weight: 1,
        models: "",
        source: "inline",
        inline_keys: ["sk-a", "sk-b"],
        pool_id: null,
      },
    ]);
  });

  it("pool 组：只带 pool_id（数字），inline_keys 为空数组", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [{ name: "池组", weight: 2, models: "gpt-4o", source: "pool", inlineKeys: "", poolId: "7" }],
      })
    );
    expect(payload.groups).toEqual([
      { name: "池组", weight: 2, models: "gpt-4o", source: "pool", inline_keys: [], pool_id: 7 },
    ]);
  });

  it("inline 组 textarea 只有空行/空白时 inline_keys 为空数组（后端 normalizeInlineKeys 兜底校验）", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [{ name: "空组", weight: 1, models: "", source: "inline", inlineKeys: "\n  \n", poolId: "" }],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({ source: "inline", inline_keys: [] });
  });
});

describe("detailToFormValues - 详情回填（AC-1/AC-5）", () => {
  it("inline 组回填明文 keys（换行拼接），不含 api_key 表单键", () => {
    const values = detailToFormValues(detail());

    expect(values.name).toBe("p-detail");
    expect(values.base_url).toBe("https://api.example.com/v1");
    expect(values.protocols).toEqual(["openai"]);
    expect(values.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);
    expect(values.groups).toEqual([
      { name: "默认组", weight: 1, models: "", source: "inline", inlineKeys: "sk-plain-1\nsk-plain-2", poolId: "" },
    ]);
    // 顶层 api_key 已废除：表单值不得再出现该键（防止回写明文进 config）
    expect("api_key" in values).toBe(false);
  });

  it("pool 组回填 poolId（字符串），inlineKeys 置空", () => {
    const values = detailToFormValues(
      detail({
        Groups: [
          {
            ID: 22,
            ProviderID: 1,
            Name: "池组",
            Weight: 3,
            Models: "gpt-4o",
            Source: "pool",
            InlineKeys: [],
            PoolID: 9,
          },
        ],
      })
    );

    expect(values.groups).toEqual([
      { name: "池组", weight: 3, models: "gpt-4o", source: "pool", inlineKeys: "", poolId: "9" },
    ]);
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

  it("Protocols 为空时回退空数组（编辑侧不做 type 派生，类型勾选交给用户）", () => {
    const values = detailToFormValues(detail({ Protocols: [] }));
    expect(values.protocols).toEqual([]);
  });
});
