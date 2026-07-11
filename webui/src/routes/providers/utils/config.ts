import type { ProviderFormValues } from "../form-schema";
import { parseAllModelsFromConfig } from "@/lib/provider-models";
import { applyExtraFieldsToConfig } from "../form-fields";

type ParsedProviderConfig = {
  base_url: string;
  api_key: string;
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
    };
  } catch {
    return {
      base_url: "",
      api_key: "",
      beta: "",
      version: "",
      auth_type: "x-api-key",
      custom_models: [],
    };
  }
}

export const extractAllModels = (config: string) => parseAllModelsFromConfig(config);