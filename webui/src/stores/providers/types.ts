import type { Provider } from "@/lib/api";

export type ModelTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
};

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  testing: number;
};

export type UpstreamStatus = "loading" | "success" | "empty" | "error" | "disabled";

export const DEFAULT_BATCH_TEST_PROGRESS: BatchTestProgress = {
  total: 0,
  completed: 0,
  success: 0,
  failed: 0,
  testing: 0,
};

export type ProvidersFilters = {
  nameFilter: string;
  debouncedNameFilter: string;
  typeFilter: string;
};

export type ProvidersPagePreferences = Pick<ProvidersFilters, "nameFilter" | "typeFilter"> & {
  // Intentionally excludes sensitive UI toggles (e.g. API key visibility).
};

export type ProvidersDialogsState = {
  providerDialogOpen: boolean;
  editingProvider: Provider | null;
  deleteId: number | null;
  clearAssociationId: number | null;
  modelsOpen: boolean;
  modelsOpenId: number | null;
  allModelsOpen: boolean;
  allModelsProvider: Provider | null;
};
