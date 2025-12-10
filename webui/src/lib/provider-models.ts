import type { ProviderModel } from "./api";

const normalize = (name: string) => name.trim();

export function parseAllModelsFromConfig(config: string): string[] {
  try {
    const parsed = JSON.parse(config ?? "{}");
    const raw = Array.isArray((parsed as any).custom_models) ? (parsed as any).custom_models : [];
    const cleaned = raw
      .map((item: unknown) => (typeof item === "string" ? normalize(item) : ""))
      .filter(Boolean);
    return Array.from(new Set(cleaned));
  } catch {
    return [];
  }
}

export function buildConfigWithAllModels(config: string, models: string[]): string {
  const uniqueModels = Array.from(new Set(models.map(normalize).filter(Boolean)));
  try {
    const parsed = JSON.parse(config || "{}");
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return JSON.stringify({ ...parsed, custom_models: uniqueModels });
    }
  } catch {
    // ignore, fallback below
  }
  return JSON.stringify({ custom_models: uniqueModels });
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
