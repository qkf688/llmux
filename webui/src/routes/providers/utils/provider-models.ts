import type { Provider } from "@/lib/api";
import { extractAllModels } from "./config";

export function getAllModelsForProvider(providers: Provider[], providerId: number): string[] {
  const provider = providers.find((item) => item.ID === providerId);
  if (!provider) return [];
  return extractAllModels(provider.Config);
}

