export const buildSelectionKey = (providerId: number, modelId: string) =>
  `${providerId}::${modelId.toLowerCase()}`;

