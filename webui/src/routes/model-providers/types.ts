import type { Provider, ProviderModel } from "@/lib/api";

export type ProviderModelWithOwner = ProviderModel & {
  providerId: number;
  providerName: string;
};

export type ProviderModelSelection = {
  providerId: number;
  providerName: string;
  modelId: string;
};

export type ProviderModelGroup = {
  provider: Provider;
  models: ProviderModelWithOwner[];
};

export type BlacklistFilter = "all" | "blacklisted" | "not-blacklisted";

export type TestType = "connectivity" | "react" | "structured_output";

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  testing: number;
};

export type AssociationBatchTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
};
