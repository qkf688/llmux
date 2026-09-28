import { describe, expect, it } from "vitest";
import type { CredentialView, ProviderDetail } from "@/lib/api";
import { buildConfigFromForm, buildProviderPayload, detailToFormValues } from "./config";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";

/** 凭据视图工厂（后端 service/credview.View）：用例只关心 ID/掩码/状态，其余字段由基座补齐。
 *  用 Partial 工厂而非 `as CredentialView`——新增字段时 tsc 会在这里报错，不会静默放过。 */
function credentialView(p: Partial<CredentialView> & { ID: number; KeyMasked: string }): CredentialView {
  return {
    PoolID: null,
    GroupID: 1,
    Status: "active",
    Note: "",
    CooldownUntil: null,
    CooldownReason: "",
    FailCount: 0,
    LastUsedAt: null,
    LastProbeAt: null,
    TotalRequests: 0,
    TotalErrors: 0,
    TotalTokens: 0,
    CreatedAt: "2026-09-28T00:00:00Z",
    UpdatedAt: "2026-09-28T00:00:00Z",
    ...p,
  };
}

function formValues(overrides: Partial<ProviderFormValues> = {}): ProviderFormValues {
  return {
    ...defaultProviderFormValues,
    type: "openai",
    base_url: "https://api.example.com/v1",
    protocols: ["openai"],
    endpoints: [{ protocol: "openai", url: "", enabled: true }],
    groups: [
      {
        name: "默认组",
        weight: 1,
        source: "inline",
        inlineKeys: [
          { id: 0, key: "sk-a", masked: false },
          { id: 0, key: "sk-b", masked: false },
        ],
        poolId: "",
      },
    ],
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
        InlineKeys: [
          credentialView({ ID: 101, KeyMasked: "sk-****-1" }),
          credentialView({ ID: 102, KeyMasked: "sk-****-2" }),
        ],
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

describe("buildProviderPayload - 行形态 → InlineKeyInput（掩码保留/明文提交）", () => {
  it("payload 携带 protocols/endpoints/groups，config 内嵌无 api_key", () => {
    const payload = buildProviderPayload(formValues(), { name: "p", type: "openai" });

    expect(payload.name).toBe("p");
    expect(payload.protocols).toEqual(["openai"]);
    expect(payload.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);

    const config = JSON.parse(payload.config ?? "{}");
    expect(config).not.toHaveProperty("api_key");
    expect(config).not.toHaveProperty("_schedule");
  });

  it("明文纯新增行：按 id=0 + key 提交，pool_id 为 null", () => {
    const payload = buildProviderPayload(formValues());
    expect(payload.groups).toEqual([
      {
        id: 0,
        name: "默认组",
        weight: 1,
        source: "inline",
        inline_keys: [
          { id: 0, key: "sk-a" },
          { id: 0, key: "sk-b" },
        ],
        pool_id: null,
      },
    ]);
  });

  it("掩码行提交 key 空串按 id 保留；明文行 key 原样提交", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [
          {
            name: "默认组",
            weight: 1,
            source: "inline",
            inlineKeys: [
              { id: 101, key: "sk-****-1", masked: true },
              { id: 102, key: "sk-plain-1", masked: false },
            ],
            poolId: "",
          },
        ],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({
      inline_keys: [
        { id: 101, key: "" },
        { id: 102, key: "sk-plain-1" },
      ],
    });
  });

  it("新加的空占位/空白明文行被剔除（对齐后端 normalizeInlineKeys 空条目兜底）", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [
          {
            name: "默认组",
            weight: 1,
            source: "inline",
            inlineKeys: [
              { id: 101, key: "sk-****-1", masked: true },
              { id: undefined, key: "", masked: false },
              { id: undefined, key: "  ", masked: false },
            ],
            poolId: "",
          },
        ],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({ inline_keys: [{ id: 101, key: "" }] });
  });

  it("清空已显示行（明文态 + 空文本）= 删除该凭据，不带 id 提交；掩码行仍按 id 保留", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [
          {
            name: "默认组",
            weight: 1,
            source: "inline",
            inlineKeys: [
              { id: 101, key: "", masked: false },
              { id: 102, key: "sk-****-2", masked: true },
            ],
            poolId: "",
          },
        ],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({ inline_keys: [{ id: 102, key: "" }] });
  });

  it("pool 组：只带 pool_id（数字），inline_keys 为空数组；组 id 随行回传（就地更新键）", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [{ name: "池组", weight: 2, source: "pool", inlineKeys: [], groupId: 22, poolId: "7" }],
      })
    );
    expect(payload.groups).toEqual([
      { id: 22, name: "池组", weight: 2, source: "pool", inline_keys: [], pool_id: 7 },
    ]);
  });

  it("明文重复行原样提交（重复提示在 UI，去重语义由后端 hash 复用/新建兜底）", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [
          {
            name: "默认组",
            weight: 1,
            source: "inline",
            inlineKeys: [
              { id: 0, key: "sk-a", masked: false },
              { id: 0, key: "sk-a", masked: false },
            ],
            poolId: "",
          },
        ],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({
      inline_keys: [
        { id: 0, key: "sk-a" },
        { id: 0, key: "sk-a" },
      ],
    });
  });

  it("inline 空组（无任何行）提交空数组（后端「至少保留一个 key 或引用」兜底校验）", () => {
    const payload = buildProviderPayload(
      formValues({
        groups: [
          { name: "空组", weight: 1, source: "inline", inlineKeys: [], poolId: "" },
        ],
      })
    );
    expect(payload.groups?.[0]).toMatchObject({ source: "inline", inline_keys: [] });
  });
});

describe("detailToFormValues - 详情掩码回填（对齐号池 KeyMasked）", () => {
  it("inline 组回填掩码行（服务端掩码 + 凭据 ID，masked=true），组 ID 带入供显示拉取", () => {
    const values = detailToFormValues(detail());

    expect(values.name).toBe("p-detail");
    expect(values.base_url).toBe("https://api.example.com/v1");
    expect(values.protocols).toEqual(["openai"]);
    expect(values.endpoints).toEqual([{ protocol: "openai", url: "", enabled: true }]);
    expect(values.groups).toEqual([
      {
        name: "默认组",
        weight: 1,
        source: "inline",
        inlineKeys: [
          { id: 101, key: "sk-****-1", masked: true, status: "active", cooldownUntil: null, failCount: 0 },
          { id: 102, key: "sk-****-2", masked: true, status: "active", cooldownUntil: null, failCount: 0 },
        ],
        groupId: 21,
        poolId: "",
      },
    ]);
    // 顶层 api_key 已废除：表单值不得再出现该键（防止回写明文进 config）
    expect("api_key" in values).toBe(false);
  });

  it("pool 组回填 poolId（字符串）+ groupId，inlineKeys 置空", () => {
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

    expect(values.groups?.[0]).toMatchObject({
      name: "池组",
      weight: 3,
      source: "pool",
      inlineKeys: [],
      groupId: 22,
      poolId: "9",
    });
  });

  it("解密失败组：掩码退化 **** 回填（保留引用、保存按 id 原样保留，不再丢凭据）", () => {
    const values = detailToFormValues(
      detail({
        Groups: [
          {
            ID: 23,
            ProviderID: 1,
            Name: "失败组",
            Weight: 1,
            Models: "",
            Source: "inline",
            InlineKeys: [
              credentialView({ ID: 103, KeyMasked: "****", Status: "temp_unsched", FailCount: 3 }),
            ],
            PoolID: null,
          },
        ],
      })
    );

    expect(values.groups?.[0]).toMatchObject({
      source: "inline",
      // 状态随掩码一起回填：编辑弹窗据此显示状态徽标与「启用」（判停 key 的人工恢复入口）
      inlineKeys: [{ id: 103, key: "****", masked: true, status: "temp_unsched", failCount: 3 }],
      groupId: 23,
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
