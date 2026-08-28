import type { ProviderFormValues } from "../form-schema";
import { parseAllModelsFromConfig } from "@/lib/provider-models";
import { applyExtraFieldsToConfig } from "../form-fields";
import { parseScheduleFromConfig, type ProviderScheduleShape } from "./schedule";

type ParsedProviderConfig = {
  base_url: string;
  api_key: string;
  beta?: string;
  version?: string;
  auth_type?: string;
  custom_models: string[];
  /** S0 原型：协议/端点/分组（存于 config 的 _schedule 键） */
  schedule?: ProviderScheduleShape;
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

export function buildConfigFromForm(values: ProviderFormValues): string {
  const customModels = parseCustomModelsInput(values.custom_models);
  const baseConfig: Record<string, unknown> = {
    base_url: values.base_url,
    api_key: values.api_key,
  };

  applyExtraFieldsToConfig(values.type, values, baseConfig);

  if (customModels.length > 0) {
    baseConfig.custom_models = customModels;
  }

  // S0 原型：调度配置随 config 保存（后端对 config 为自由 JSON，原样存取）。
  // S6 起改为独立 DTO 字段（protocols/endpoints/groups），此键废弃。
  baseConfig._schedule = {
    protocols: values.protocols ?? [],
    endpoints: (values.endpoints ?? []).map((e) => ({
      protocol: e.protocol,
      url: e.url,
      enabled: e.enabled,
    })),
    groups: (values.groups ?? []).map((g) => ({
      name: g.name,
      weight: g.weight,
      models: g.models,
      source: g.source,
      inlineKeys: g.inlineKeys,
      poolId: g.poolId,
    })),
  };

  return JSON.stringify(baseConfig);
}

export function parseConfigToForm(config: string): ParsedProviderConfig {
  try {
    const parsed = JSON.parse(config);
    return {
      base_url: parsed.base_url || "",
      api_key: parsed.api_key || "",
      beta: parsed.beta || "",
      version: parsed.version || "",
      auth_type: parsed.auth_type || "x-api-key",
      custom_models: Array.isArray(parsed.custom_models)
        ? parsed.custom_models.filter((item: unknown) => typeof item === "string" && item.trim() !== "")
        : [],
      schedule: parseScheduleFromConfig(config) ?? undefined,
    };
  } catch {
    return {
      base_url: "",
      api_key: "",
      beta: "",
      version: "",
      auth_type: "x-api-key",
      custom_models: [],
      schedule: undefined,
    };
  }
}

export const extractAllModels = (config: string) => parseAllModelsFromConfig(config);