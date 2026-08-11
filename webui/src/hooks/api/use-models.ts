import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getModels,
  createModel,
  updateModel,
  deleteModel,
  batchDeleteModels,
  batchUpdateModels,
  type Model,
} from '@/lib/api';

export const modelKeys = {
  all: ['models'] as const,
  lists: () => [...modelKeys.all, 'list'] as const,
  list: () => [...modelKeys.lists()] as const,
};

export function useModels() {
  return useQuery({
    queryKey: modelKeys.list(),
    queryFn: getModels,
  });
}

export function useCreateModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createModel,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export function useUpdateModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Parameters<typeof updateModel>[1] }) =>
      updateModel(id, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export function useDeleteModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteModel,
    // 乐观更新：立即从缓存中移除被删项，让 AnimatePresence 稳定接管退场动画
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: modelKeys.lists() });
      const previous = qc.getQueriesData<Model[]>({ queryKey: modelKeys.lists() });
      qc.setQueriesData<Model[]>(
        { queryKey: modelKeys.lists() },
        (old) => old?.filter((m) => m.ID !== id),
      );
      return { previous };
    },
    onError: (_err, _id, context) => {
      // 请求失败回滚到乐观更新前的快照
      if (context?.previous) {
        for (const [key, data] of context.previous) {
          qc.setQueryData(key, data);
        }
      }
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export function useBatchDeleteModels() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: batchDeleteModels,
    // 与单条删除同理：批量删除也先从缓存移除，否则多行同时退场会被 refetch 打断
    onMutate: async (ids: number[]) => {
      await qc.cancelQueries({ queryKey: modelKeys.lists() });
      const previous = qc.getQueriesData<Model[]>({ queryKey: modelKeys.lists() });
      const removing = new Set(ids);
      qc.setQueriesData<Model[]>(
        { queryKey: modelKeys.lists() },
        (old) => old?.filter((m) => !removing.has(m.ID)),
      );
      return { previous };
    },
    onError: (_err, _ids, context) => {
      if (context?.previous) {
        for (const [key, data] of context.previous) {
          qc.setQueryData(key, data);
        }
      }
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export function useBatchUpdateModels() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: batchUpdateModels,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export type { Model };
