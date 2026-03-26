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
