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
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelKeys.all });
    },
  });
}

export function useBatchDeleteModels() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: batchDeleteModels,
    onSuccess: () => {
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
