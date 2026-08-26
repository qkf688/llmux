/**
 * Models 页面选择逻辑：选中模型集合、全选/单选、选中项清理。
 */
import { useEffect } from "react";
import type { Model } from "@/lib/api";

interface UseModelsSelectionParams {
  models: Model[];
  selectedIds: number[];
  setSelectedIds: (updater: (previous: number[]) => number[]) => void;
}

export function useModelsSelection({ models, selectedIds, setSelectedIds }: UseModelsSelectionParams) {
  // 选中项清理：models 变化后过滤掉已不存在的 id
  useEffect(() => {
    setSelectedIds((previous) => {
      if (previous.length === 0) {
        return previous;
      }

      const validIdSet = new Set(models.map((model) => model.ID));
      const next = previous.filter((id) => validIdSet.has(id));
      return next.length === previous.length ? previous : next;
    });
  }, [models, setSelectedIds]);

  const isAllSelected = models.length > 0 && selectedIds.length === models.length;
  const isPartialSelected = selectedIds.length > 0 && selectedIds.length < models.length;

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(() => models.map((model) => model.ID));
      return;
    }

    setSelectedIds(() => []);
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((previous) => {
      if (checked) {
        return previous.includes(id) ? previous : [...previous, id];
      }

      return previous.filter((selectedId) => selectedId !== id);
    });
  };

  return {
    isAllSelected,
    isPartialSelected,
    handleSelectAll,
    handleSelectOne,
  };
}