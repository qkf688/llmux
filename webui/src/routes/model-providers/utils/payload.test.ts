import { describe, expect, it } from "vitest";
import type { FormValues } from "../form-schema";
import { buildAssociationPayload } from "./payload";

function valuesWithHeaders(headers: { key: string; value: string }[]): FormValues {
  return {
    model_id: 1,
    provider_model: "pm",
    provider_id: 1,
    tool_call: true,
    structured_output: false,
    image: false,
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
