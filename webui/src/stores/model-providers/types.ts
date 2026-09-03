import type { ModelProviderTestResult } from "@/lib/api";

export type {
  ProviderModelGroup,
  ProviderModelWithOwner,
} from "@/lib/provider-models";

export type ProviderModelSelection = {
  providerId: number;
  providerName: string;
  modelId: string;
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

export type ReactTestResultState = {
  loading: boolean;
  messages: string;
  success: boolean | null;
  error: string | null;
};

export type ModelProvidersPagePreferences = {
  searchKeyword: string;
  selectedProviderType: string;
  selectedProviderFilter: string;
  selectedStatusFilter: string;
  filterPanelOpen: boolean;
  operationScope: "current" | "all";
};

export type InlineTestResultMap = Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;

