import type { Provider, ProviderModel } from "@/lib/api";

export type ProviderModelWithOwner = ProviderModel & {
  providerId: number;
  providerName: string;
};

export type ProviderModelGroup = {
  provider: Provider;
  models: ProviderModelWithOwner[];
};

export type ValueRange = {
  min: number;
  max: number;
};
