import type { ProviderFormValues } from "./form-schema";

export type ProviderExtraFieldName = "version" | "beta" | "auth_type";

export type ProviderExtraField = {
  name: ProviderExtraFieldName;
  label: string;
  kind: "text" | "select";
  options?: { value: string; label: string }[];
  placeholder?: string;
  /** 写入 config 时的默认值（表单空时使用） */
  defaultValue?: string;
  /** 是否写入 API 顶层 auth_type（仅 auth_type） */
  submitAsTopLevel?: boolean;
};

/**
 * 按 provider type 描述额外表单字段。
 * openai / openai-res 等无额外字段；扩展新 type 只改本表。
 */
export const providerTypeExtras: Record<string, ProviderExtraField[]> = {
  anthropic: [
    {
      name: "version",
      label: "Version",
      kind: "text",
      placeholder: "2023-06-01",
      defaultValue: "2023-06-01",
    },
    {
      name: "beta",
      label: "Beta（可选）",
      kind: "text",
      placeholder: "可选的 beta 标识",
      defaultValue: "",
    },
    {
      name: "auth_type",
      label: "认证方式",
      kind: "select",
      defaultValue: "x-api-key",
      submitAsTopLevel: true,
      options: [
        { value: "x-api-key", label: "x-api-key（Anthropic 官方）" },
        { value: "bearer", label: "Authorization: Bearer（兼容第三方）" },
      ],
    },
  ],
};

export function getProviderExtraFields(type: string): ProviderExtraField[] {
  return providerTypeExtras[type] ?? [];
}

export function hasProviderExtraFields(type: string): boolean {
  return getProviderExtraFields(type).length > 0;
}

/** 将 schema 中的额外字段写入 config 对象 */
export function applyExtraFieldsToConfig(
  type: string,
  values: ProviderFormValues,
  config: Record<string, unknown>,
): void {
  for (const field of getProviderExtraFields(type)) {
    const raw = values[field.name];
    const value =
      typeof raw === "string" && raw.trim() !== ""
        ? raw
        : (field.defaultValue ?? "");
    config[field.name] = value;
  }
}

/** 应用模板时，按 schema 回填额外字段默认值 */
export function getExtraFieldDefaultsFromParsed(
  type: string,
  parsed: Partial<Record<ProviderExtraFieldName, string | undefined>>,
): Partial<Record<ProviderExtraFieldName, string>> {
  const result: Partial<Record<ProviderExtraFieldName, string>> = {};
  for (const field of getProviderExtraFields(type)) {
    const fromTemplate = parsed[field.name];
    result[field.name] =
      (fromTemplate && fromTemplate.trim() !== ""
        ? fromTemplate
        : field.defaultValue) ?? "";
  }
  return result;
}

/** 提交时是否需要顶层 auth_type */
export function resolveTopLevelAuthType(
  type: string,
  values: ProviderFormValues,
): string | undefined {
  const field = getProviderExtraFields(type).find((f) => f.submitAsTopLevel && f.name === "auth_type");
  if (!field) return undefined;
  const raw = values.auth_type;
  if (typeof raw === "string" && raw.trim() !== "") return raw;
  return field.defaultValue ?? "x-api-key";
}