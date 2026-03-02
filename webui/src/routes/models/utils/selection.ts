import type { Model } from "@/lib/api";
import type { ValueRange } from "../types";

const EMPTY_RANGE: ValueRange = { min: 0, max: 0 };

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

const buildRange = (values: number[]): ValueRange => {
  if (values.length === 0) {
    return EMPTY_RANGE;
  }

  return {
    min: Math.min(...values),
    max: Math.max(...values),
  };
};

export const calculateSelectedRanges = (
  models: Model[]
): { maxRetryRange: ValueRange; timeOutRange: ValueRange } => ({
  maxRetryRange: buildRange(models.map((model) => model.MaxRetry)),
  timeOutRange: buildRange(models.map((model) => model.TimeOut)),
});
