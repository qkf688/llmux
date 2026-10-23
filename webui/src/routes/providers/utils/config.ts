import type { ProviderDetail, ProviderWritePayload } from "@/lib/api";
import type { ProviderFormValues } from "../form-schema";
import { applyExtraFieldsToConfig, getActiveProviderExtras } from "../form-fields";

type ParsedProviderConfig = {
  base_url: string;
  beta?: string;
  version?: string;
  auth_type?: string;
  openai_auth_type?: string;
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
 * S6 起不含 api_key / _schedule：凭据走号池、协议/端点/分组走独立 DTO 字段。
 * 语法上不写两者，避免把明文 key 或已废弃的调度结构落进 config。
 */
export function buildConfigFromForm(values: ProviderFormValues): string {
  const customModels = parseCustomModelsInput(values.custom_models);
  const baseConfig: Record<string, unknown> = {
    base_url: values.base_url,
  };

  applyExtraFieldsToConfig(values.type, values.protocols, values, baseConfig);

  if (customModels.length > 0) {
    baseConfig.custom_models = customModels;
  }

  return JSON.stringify(baseConfig);
}

/**
 * 由表单构建 ProviderWritePayload：config + 顶层标量 + 结构化 children。
 * 表单提交（创建/编辑保存）全量携带 protocols/endpoints/groups，走后端结构化四表同步。
 * 分组只带 pool_id——凭据一律归属号池，本表单不再写入任何 key。
 */
export function buildProviderPayload(values: ProviderFormValues, provider: ProviderWritePayload = {}): ProviderWritePayload {
  const endpoints = (values.endpoints ?? []).map((e) => ({
    protocol: e.protocol,
    url: e.url,
    enabled: e.enabled,
  }));

  const groups = (values.groups ?? []).map((g) => ({
    id: g.groupId ?? 0,
    name: g.name,
    weight: g.weight,
    pool_id: g.poolId ? Number(g.poolId) : null,
  }));

  return {
    ...provider,
    config: buildConfigFromForm(values),
    protocols: values.protocols ?? [],
    endpoints,
    groups,
  };
}

function parseConfigToAdapterParts(
  config: string,
): Pick<ParsedProviderConfig, "base_url" | "beta" | "version" | "auth_type" | "openai_auth_type" | "custom_models"> {
  try {
    const parsed = JSON.parse(config);
    return {
      base_url: (parsed && typeof parsed === "object" ? parsed.base_url : "") || "",
      beta: parsed?.beta || "",
      version: parsed?.version || "",
      // 空串=config 未写：认证方言默认值随类型不同（anthropic=x-api-key、
      // openai 系=bearer，4b 拆键），此处不得写死任何一家的默认，交由带类型的调用方回填。
      auth_type: parsed?.auth_type || "",
      openai_auth_type: parsed?.openai_auth_type || "",
      custom_models: Array.isArray(parsed?.custom_models)
        ? parsed.custom_models.filter((item: unknown) => typeof item === "string" && item.trim() !== "")
        : [],
    };
  } catch {
    return { base_url: "", beta: "", version: "", auth_type: "", openai_auth_type: "", custom_models: [] };
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
  // config 缺方言键（存量行）时回退「激活字段定义」的类型默认，不写死任何一家
  // （4b 拆键：auth_type=anthropic、openai_auth_type=openai 系）。
  const extras = getActiveProviderExtras(detail.Type, detail.Protocols);
  const defaultFor = (name: "auth_type" | "openai_auth_type") =>
    extras.find((f) => f.name === name)?.defaultValue ?? "";
  return {
    name: detail.Name,
    type: detail.Type,
    base_url: parts.base_url,
    beta: parts.beta || "",
    version: parts.version || "",
    auth_type: parts.auth_type || defaultFor("auth_type"),
    openai_auth_type: parts.openai_auth_type || defaultFor("openai_auth_type"),
    console: detail.Console || "",
    custom_models: parts.custom_models.join("\n"),
    proxy: detail.Proxy || "",
    model_endpoint: detail.ModelEndpoint ?? true,
    model_filter_enabled: detail.ModelFilterEnabled ?? false,
    protocols: detail.Protocols?.length ? detail.Protocols : [],
    endpoints: (detail.Endpoints ?? []).map((e) => ({ protocol: e.Protocol, url: e.URL, enabled: e.Enabled })),
    // 分组回填：凭据一律归属号池，只回填 poolId（空 = 存量异常数据，前端仍要求用户重选）。
    groups: (detail.Groups ?? []).map((g) => ({
      name: g.Name,
      weight: g.Weight,
      groupId: g.ID,
      poolId: g.PoolID != null ? String(g.PoolID) : "",
    })),
  };
}
