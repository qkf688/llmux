import { SUPPORTED_TYPE_OPTIONS } from "./form-schema";
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

/**
 * 按「主类型 + 已勾选协议」合并激活的额外字段（按字段名去重）。
 * 主类型决定了 providers.New 分发，但协议端点按端点协议实例化 provider——
 * 主类型是 openai、勾选 anthropic 协议时，anthropic 端点同样消费 config 的
 * version/beta/auth_type，因此这些字段的显示与写入不能只看 type。
 * protocols → type 反查复用 SUPPORTED_TYPE_OPTIONS（type↔protocol 一一对应），
 * 不另维护映射表避免漂移。
 */
export function getActiveProviderExtras(
  type: string,
  protocols: string[] | undefined,
): ProviderExtraField[] {
  const activeTypes = new Set<string>();
  if (type) {
    activeTypes.add(type);
  }
  for (const protocol of protocols ?? []) {
    const option = SUPPORTED_TYPE_OPTIONS.find((o) => o.protocol === protocol);
    if (option) {
      activeTypes.add(option.type);
    }
  }
  const fields = new Map<string, ProviderExtraField>();
  for (const t of activeTypes) {
    for (const field of providerTypeExtras[t] ?? []) {
      fields.set(field.name, field);
    }
  }
  return [...fields.values()];
}

/** 将 schema 中的额外字段（主类型 + 已勾选协议激活的部分）写入 config 对象 */
export function applyExtraFieldsToConfig(
  type: string,
  protocols: string[] | undefined,
  values: ProviderFormValues,
  config: Record<string, unknown>,
): void {
  for (const field of getActiveProviderExtras(type, protocols)) {
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