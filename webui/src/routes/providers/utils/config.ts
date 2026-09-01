import type { ProviderDetail, ProviderWritePayload } from "@/lib/api";
import { parseAllModelsFromConfig } from "@/lib/provider-models";
import type { ProviderFormValues } from "../form-schema";
import { applyExtraFieldsToConfig } from "../form-fields";

type ParsedProviderConfig = {
  base_url: string;
  beta?: string;
  version?: string;
  auth_type?: string;
  custom_models: string[];
};

export function parseCustomModelsInput(input?: string): string[] {
  if (!input) {
    return [];
  }

  return input
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

/**
 * 由表单构建 config JSON（仅 adapter 字段 + custom_models）。
 * S6 起不含 api_key / _schedule：凭据走分组内联/号池、协议/端点/分组走独立 DTO 字段。
 * 语法上不写两者，避免把明文 key 或已废弃的调度结构落进 config。
 */
export function buildConfigFromForm(values: ProviderFormValues): string {
  const customModels = parseCustomModelsInput(values.custom_models);
  const baseConfig: Record<string, unknown> = {
    base_url: values.base_url,
  };

  applyExtraFieldsToConfig(values.type, values, baseConfig);

  if (customModels.length > 0) {
    baseConfig.custom_models = customModels;
  }

  return JSON.stringify(baseConfig);
}

/**
 * 由表单构建 ProviderWritePayload：config + 顶层标量 + 结构化 children。
 * 表单提交（创建/编辑保存）全量携带 protocols/endpoints/groups，走后端结构化四表同步。
 * inline 组的 keys 拆成数组（去空/去占位，对齐后端 normalizeInlineKeys）；pool 组只带 pool_id。
 */
export function buildProviderPayload(values: ProviderFormValues, provider: ProviderWritePayload = {}): ProviderWritePayload {
  const endpoints = (values.endpoints ?? []).map((e) => ({
    protocol: e.protocol,
    url: e.url,
    enabled: e.enabled,
  }));

  const groups = (values.groups ?? []).map((g) =>
    g.source === "pool"
      ? {
          name: g.name,
          weight: g.weight,
          models: g.models,
          source: "pool" as const,
          inline_keys: [],
          pool_id: g.poolId ? Number(g.poolId) : null,
        }
      : {
          name: g.name,
          weight: g.weight,
          models: g.models,
          source: "inline" as const,
          inline_keys: parseCustomModelsInput(g.inlineKeys),
          pool_id: null,
        }
  );

  return {
    ...provider,
    config: buildConfigFromForm(values),
    protocols: values.protocols ?? [],
    endpoints,
    groups,
  };
}

function parseConfigToAdapterParts(config: string): Pick<ParsedProviderConfig, "base_url" | "beta" | "version" | "auth_type" | "custom_models"> {
  try {
    const parsed = JSON.parse(config);
    return {
      base_url: (parsed && typeof parsed === "object" ? parsed.base_url : "") || "",
      beta: parsed?.beta || "",
      version: parsed?.version || "",
      auth_type: parsed?.auth_type || "x-api-key",
      custom_models: Array.isArray(parsed?.custom_models)
        ? parsed.custom_models.filter((item: unknown) => typeof item === "string" && item.trim() !== "")
        : [],
    };
  } catch {
    return { base_url: "", beta: "", version: "", auth_type: "x-api-key", custom_models: [] };
  }
}

/**
 * 解析 config JSON 的 adapter 字段（base_url / version / beta / auth_type / custom_models）。
 * 供模板套用（applyProviderTemplateDefaults）读取模板默认值。不含 api_key / _schedule——
 * 凭据与协议结构在 S6 已迁出 config。
 */
export function parseConfigToForm(config: string): ParsedProviderConfig {
  return parseConfigToAdapterParts(config);
}

/** 从详情 API 回填表单。端点/分组来自详情展开（不含 Config._schedule）。 */
export function detailToFormValues(detail: ProviderDetail): ProviderFormValues {
  const parts = parseConfigToAdapterParts(detail.Config);
  return {
    name: detail.Name,
    type: detail.Type,
    base_url: parts.base_url,
    beta: parts.beta || "",
    version: parts.version || "",
    auth_type: parts.auth_type || "x-api-key",
    console: detail.Console || "",
    custom_models: parts.custom_models.join("\n"),
    proxy: detail.Proxy || "",
    model_endpoint: detail.ModelEndpoint ?? true,
    model_filter_enabled: detail.ModelFilterEnabled ?? false,
    protocols: detail.Protocols?.length ? detail.Protocols : [],
    endpoints: (detail.Endpoints ?? []).map((e) => ({ protocol: e.Protocol, url: e.URL, enabled: e.Enabled })),
    groups: (detail.Groups ?? []).map((g) =>
      g.Source === "pool"
        ? { name: g.Name, weight: g.Weight, models: g.Models, source: "pool" as const, inlineKeys: "", poolId: g.PoolID != null ? String(g.PoolID) : "" }
        : { name: g.Name, weight: g.Weight, models: g.Models, source: "inline" as const, inlineKeys: g.InlineKeys.join("\n"), poolId: "" }
    ),
  };
}

export const extractAllModels = (config: string) => parseAllModelsFromConfig(config);