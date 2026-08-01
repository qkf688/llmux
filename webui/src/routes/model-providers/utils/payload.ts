import type { FormValues } from "../form-schema";

export const buildAssociationPayload = (
  values: FormValues,
  overrides?: {
    providerId?: number;
    providerModel?: string;
  }
) => {
  const headers: Record<string, string> = {};
  (values.customer_headers || []).forEach(({ key, value }) => {
    const trimmedKey = key.trim();
    if (trimmedKey) {
      headers[trimmedKey] = value ?? "";
    }
  });

  return {
    model_id: values.model_id,
    provider_name: (overrides?.providerModel ?? values.provider_name) || "",
    provider_id: overrides?.providerId ?? values.provider_id,
    tool_call: values.tool_call,
    structured_output: values.structured_output,
    image: values.image,
    with_header: values.with_header,
    customer_headers: headers,
    weight: values.weight,
    priority: values.priority,
    max_tokens: values.max_tokens ?? 0,
  };
};

