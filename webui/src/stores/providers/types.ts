import type { Provider } from "@/lib/api";

export type ModelTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
  /** 用户主动中止（批量取消 / 切走页面）。与「失败」分开：中止不证明模型不可用，
   *  「选择失败的模型」之类的下游动作必须把它排除在外。 */
  cancelled?: boolean;
};

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  /** 被中止的项数。与 failed 分开计数，失败率不被取消污染。 */
  cancelled: number;
  testing: number;
};

export type UpstreamStatus = "loading" | "success" | "empty" | "error" | "disabled";

export type AllModelsTypeFilter = "all" | "upstream" | "custom";

export const DEFAULT_BATCH_TEST_PROGRESS: BatchTestProgress = {
  total: 0,
  completed: 0,
  success: 0,
  failed: 0,
  cancelled: 0,
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
  modelsOpen: boolean;
  modelsOpenId: number | null;
  allModelsOpen: boolean;
  allModelsProvider: Provider | null;
};
