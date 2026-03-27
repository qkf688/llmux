export type VirtualModelsBatchDefaults = {
  priority: number;
  weight: number;
  enabled: boolean;
};

export const DEFAULT_VIRTUAL_MODELS_BATCH: VirtualModelsBatchDefaults = {
  priority: 10,
  weight: 5,
  enabled: true,
};

export type VirtualModelsPagePreferences = {
  modelSearchQuery: string;
  providerSearchQuery: string;
  batchPriority: number;
  batchWeight: number;
  batchEnabled: boolean;
};

export const DEFAULT_VIRTUAL_MODELS_PAGE_PREFERENCES: VirtualModelsPagePreferences = {
  modelSearchQuery: "",
  providerSearchQuery: "",
  batchPriority: DEFAULT_VIRTUAL_MODELS_BATCH.priority,
  batchWeight: DEFAULT_VIRTUAL_MODELS_BATCH.weight,
  batchEnabled: DEFAULT_VIRTUAL_MODELS_BATCH.enabled,
};
