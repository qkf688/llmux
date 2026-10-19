import { describe, expect, it } from "vitest";
import type { FormValues } from "../form-schema";
import { buildAssociationPayload } from "./payload";

function valuesWithHeaders(headers: { key: string; value: string }[]): FormValues {
  return {
    model_id: 1,
    provider_id: 1,
    capabilities_mode: "custom",
    tool_call: true,
    structured_output: false,
    image: true,
    with_header: false,
    weight: 10,
    priority: 10,
    customer_headers: headers,
    supports_thinking: "inherit",
    thinking_levels_mode: "inherit",
    thinking_levels_custom: [],
    protocols_mode: "inherit",
    protocols: [],
    primary_protocol: "",
  };
}

describe("buildAssociationPayload 协议三态", () => {
  it("集合继承时不提交集合字段（后端写 NULL=继承）", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      protocols_mode: "inherit",
      protocols: ["anthropic"],
      primary_protocol: "",
    });
    expect(payload.protocols).toBeUndefined();
    expect(payload.primary_protocol).toBeUndefined();
  });

  it("集合继承但单独指定主格式时只提交主格式（两项是独立三态）", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      protocols_mode: "inherit",
      protocols: [],
      primary_protocol: "anthropic",
    });
    expect(payload.protocols).toBeUndefined();
    expect(payload.primary_protocol).toBe("anthropic");
  });

  it("自定义时集合与主格式都提交", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      protocols_mode: "custom",
      protocols: ["responses", "anthropic"],
      primary_protocol: "anthropic",
    });
    expect(payload.protocols).toEqual(["responses", "anthropic"]);
    expect(payload.primary_protocol).toBe("anthropic");
  });
});

describe("buildAssociationPayload customer_headers 折叠（与 api-keys 共用 @/lib/header-rows 单源）", () => {
  it("只差大小写的同名行按后填者赢合并（键名取后者）", () => {
    const payload = buildAssociationPayload(
      valuesWithHeaders([
        { key: "x-app", value: "1" },
        { key: "X-App", value: "2" },
      ]),
    );
    expect(payload.customer_headers).toEqual({ "X-App": "2" });
  });

  it("空名行跳过，不当错误、不上行", () => {
    const payload = buildAssociationPayload(
      valuesWithHeaders([
        { key: "  ", value: "x" },
        { key: "X-Real", value: "1" },
      ]),
    );
    expect(payload.customer_headers).toEqual({ "X-Real": "1" });
  });

  it("三态字段 inherit 不携带键（后端置 NULL=继承）", () => {
    const payload = buildAssociationPayload(valuesWithHeaders([]));
    expect(payload.supports_thinking).toBeUndefined();
    expect(payload.thinking_levels).toBeUndefined();
    expect(payload.customer_headers).toEqual({});
  });
});

// buildPayload 刻意不产出 provider_model：Create 由调用点逐条注入（每条选中模型的
// 上游名各不同），Update 则完全不发——后端契约是「缺省=不改」，显式发空串会被
// 写入闸 400（见 docs/architecture/modules/associations.md 的 provider_model 写入闸）。
describe("buildAssociationPayload 不产出 provider_model", () => {
  it("Create 路径的 payload 里没有该键，由调用点补", () => {
    expect(buildAssociationPayload(valuesWithHeaders([]))).not.toHaveProperty("provider_model");
  });

  it("带 providerId 覆盖时同样不产出（模型名不来自表单）", () => {
    const payload = buildAssociationPayload(valuesWithHeaders([]), { providerId: 7 });
    expect(payload).not.toHaveProperty("provider_model");
    expect(payload.provider_id).toBe(7);
  });
});

// 能力总控的传输面：capabilities_mode 决定三项发不发、发什么。
// 走错的后果是静默的——发错了只是能力配置不对，没有编译期或运行期报错。
describe("buildAssociationPayload 能力总控", () => {
  it("总控=继承 → 三项一律不发（后端写 NULL=继承模型真值）", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      capabilities_mode: "inherit",
      // 即便表单里勾着东西，继承模式下也必须忽略——否则勾选框会变成假动作
      tool_call: true,
      structured_output: true,
      image: true,
    });
    expect(payload.tool_call).toBeUndefined();
    expect(payload.structured_output).toBeUndefined();
    expect(payload.image).toBeUndefined();
  });

  it("总控=自定义 → 三项按勾选原样发出（含 false）", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      capabilities_mode: "custom",
      tool_call: true,
      structured_output: false,
      image: false,
    });
    expect(payload.tool_call).toBe(true);
    expect(payload.structured_output).toBe(false);
    expect(payload.image).toBe(false);
  });

  it("继承与显式关闭产生不同 payload，不能混同", () => {
    const inherited = buildAssociationPayload({
      ...valuesWithHeaders([]),
      capabilities_mode: "inherit",
      tool_call: false,
    });
    const disabled = buildAssociationPayload({
      ...valuesWithHeaders([]),
      capabilities_mode: "custom",
      tool_call: false,
    });
    expect(inherited.tool_call).not.toBe(disabled.tool_call);
    expect(disabled.tool_call).toBe(false);
  });
});
