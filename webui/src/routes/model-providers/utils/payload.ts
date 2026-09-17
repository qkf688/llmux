import { rowsToHeaders } from "@/lib/header-rows";
import type { FormValues } from "../form-schema";
import { fromTriState } from "../utils/tri-state";

export const buildAssociationPayload = (
  values: FormValues,
  overrides?: {
    providerId?: number;
    providerModel?: string;
  }
) => {
  // 折叠语义与 api-keys 弹窗共用单源（@/lib/header-rows）；语法/撞名的最终判定
  // 在后端写入闸 common/outboundheaders.go，两域同一口径（4a）。
  const headers = rowsToHeaders(values.customer_headers || []);

  return {
    model_id: values.model_id,
    provider_model: (overrides?.providerModel ?? values.provider_model) || "",
    provider_id: overrides?.providerId ?? values.provider_id,
    tool_call: values.tool_call,
    structured_output: values.structured_output,
    image: values.image,
    with_header: values.with_header,
    customer_headers: headers,
    weight: values.weight,
    priority: values.priority,
    max_tokens: values.max_tokens ?? 0,
    // 三态转换集中在 tri-state.ts：inherit → undefined（不传该字段，后端置 NULL）
    supports_thinking: fromTriState(values.supports_thinking),
    // thinking_levels 三态：inherit → undefined（不传，后端置 NULL=继承）
    // custom → 传数组（空数组=不约束，非空=override 白名单）
    thinking_levels:
      values.thinking_levels_mode === "inherit" ? undefined : values.thinking_levels_custom ?? [],
  };
};

