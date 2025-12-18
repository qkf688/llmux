import type { ProviderModel } from "./api";

const normalize = (name: string) => name.trim();

export function parseAllModelsFromConfig(config: string): string[] {
  try {
    const parsed = JSON.parse(config ?? "{}");
    const upstream = Array.isArray((parsed as any).upstream_models) ? (parsed as any).upstream_models : [];
    const custom = Array.isArray((parsed as any).custom_models) ? (parsed as any).custom_models : [];
    const all = [...upstream, ...custom]
      .map((item: unknown) => (typeof item === "string" ? normalize(item) : ""))
      .filter(Boolean);
    return Array.from(new Set(all));
  } catch {
    return [];
  }
}

export function parseUpstreamModelsFromConfig(config: string): string[] {
  try {
    const parsed = JSON.parse(config ?? "{}");
    const raw = Array.isArray((parsed as any).upstream_models) ? (parsed as any).upstream_models : [];
    return raw.map((item: unknown) => (typeof item === "string" ? normalize(item) : "")).filter(Boolean);
  } catch {
    return [];
  }
}

export function parseCustomModelsFromConfig(config: string): string[] {
  try {
    const parsed = JSON.parse(config ?? "{}");
    const raw = Array.isArray((parsed as any).custom_models) ? (parsed as any).custom_models : [];
    return raw.map((item: unknown) => (typeof item === "string" ? normalize(item) : "")).filter(Boolean);
  } catch {
    return [];
  }
}

export function buildConfigWithModels(config: string, upstreamModels: string[], customModels: string[]): string {
  const uniqueUpstream = Array.from(new Set(upstreamModels.map(normalize).filter(Boolean)));
  const uniqueCustom = Array.from(new Set(customModels.map(normalize).filter(Boolean)));
  try {
    const parsed = JSON.parse(config || "{}");
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return JSON.stringify({ ...parsed, upstream_models: uniqueUpstream, custom_models: uniqueCustom });
    }
  } catch {
    // ignore, fallback below
  }
  return JSON.stringify({ upstream_models: uniqueUpstream, custom_models: uniqueCustom });
}

export function buildConfigWithAllModels(config: string, models: string[]): string {
  const upstream = parseUpstreamModelsFromConfig(config);
  const upstreamSet = new Set(upstream);
  const newCustom = models.filter(m => !upstreamSet.has(m));
  return buildConfigWithModels(config, upstream, newCustom);
}

export function toProviderModelList(models: string[]): ProviderModel[] {
  const now = Date.now();
  return models.map((model) => ({
    id: model,
    object: "cached",
    created: Math.floor(now / 1000),
    owned_by: "local",
  }));
}
