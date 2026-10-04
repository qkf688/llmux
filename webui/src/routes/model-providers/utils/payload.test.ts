import { describe, expect, it } from "vitest";
import type { FormValues } from "../form-schema";
import { buildAssociationPayload } from "./payload";

function valuesWithHeaders(headers: { key: string; value: string }[]): FormValues {
  return {
    model_id: 1,
    provider_model: "pm",
    provider_id: 1,
    tool_call: "true",
    structured_output: "false",
    image: "inherit",
    with_header: false,
    weight: 10,
    priority: 10,
    customer_headers: headers,
    supports_thinking: "inherit",
    thinking_levels_mode: "inherit",
    thinking_levels_custom: [],
  };
}

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

// 能力三态是本次「模型层补真值 + 关联层留 override」的传输面：
// inherit 必须发 undefined（JSON.stringify 会丢掉 undefined 键 → 后端收不到 → 写 NULL 继承），
// 而 true/false 必须原样发出去。四个值各自可区分，才不会把继承静默改写成 override。
describe("buildAssociationPayload 能力三态", () => {
  it("inherit → undefined（不传键，后端写 NULL=继承模型真值）", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      tool_call: "inherit",
      structured_output: "inherit",
      image: "inherit",
    });
    expect(payload.tool_call).toBeUndefined();
    expect(payload.structured_output).toBeUndefined();
    expect(payload.image).toBeUndefined();
  });

  it("true/false → 原样传出（override），不被继承语义吞掉", () => {
    const payload = buildAssociationPayload({
      ...valuesWithHeaders([]),
      tool_call: "true",
      structured_output: "false",
      image: "inherit",
    });
    expect(payload.tool_call).toBe(true);
    expect(payload.structured_output).toBe(false);
    expect(payload.image).toBeUndefined();
  });

  it("override=false 是有效值，不能与 inherit 混同", () => {
    const inherited = buildAssociationPayload({
      ...valuesWithHeaders([]),
      tool_call: "inherit",
    });
    const disabled = buildAssociationPayload({
      ...valuesWithHeaders([]),
      tool_call: "false",
    });
    expect(inherited.tool_call).not.toBe(disabled.tool_call);
  });
});
