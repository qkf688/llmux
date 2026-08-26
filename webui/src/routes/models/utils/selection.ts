import type { Model } from "@/lib/api";

export const filterModelsByName = (models: Model[], keyword: string): Model[] => {
  const normalizedKeyword = keyword.trim().toLowerCase();
  if (!normalizedKeyword) {
    return models;
  }

  return models.filter((model) => model.Name.toLowerCase().includes(normalizedKeyword));
};

export const collectSelectedModels = (models: Model[], selectedIds: number[]): Model[] => {
  const selectedSet = new Set(selectedIds);
  return models.filter((model) => selectedSet.has(model.ID));
};
