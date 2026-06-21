import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getVirtualModels,
  createVirtualModel,
  updateVirtualModel,
  deleteVirtualModel,
  getVirtualModelMappings,
  createVirtualModelMapping,
  updateVirtualModelMapping,
  deleteVirtualModelMapping,
  batchDeleteVirtualModelMapping,
  batchCreateVirtualModelMapping,
  type VirtualModel,
  type VirtualModelMapping,
} from '@/lib/api';

export const virtualModelKeys = {
  all: ['virtual-models'] as const,
  lists: () => [...virtualModelKeys.all, 'list'] as const,
  list: () => [...virtualModelKeys.lists()] as const,
  mappings: (id: number | null) => [...virtualModelKeys.all, 'mappings', id] as const,
};

export function useVirtualModels() {
  return useQuery({
    queryKey: virtualModelKeys.list(),
    queryFn: getVirtualModels,
  });
}

export function useVMMappings(virtualModelId: number | null) {
  return useQuery({
    queryKey: virtualModelKeys.mappings(virtualModelId),
    queryFn: () => getVirtualModelMappings(virtualModelId!),
    enabled: virtualModelId !== null,
  });
}

export function useCreateVirtualModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createVirtualModel,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.all });
    },
  });
}

export function useUpdateVirtualModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Parameters<typeof updateVirtualModel>[1] }) =>
      updateVirtualModel(id, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.all });
    },
  });
}

export function useDeleteVirtualModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteVirtualModel,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.all });
    },
  });
}

export function useCreateVMMapping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ vmId, data }: { vmId: number; data: Parameters<typeof createVirtualModelMapping>[1] }) =>
      createVirtualModelMapping(vmId, data),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.mappings(vars.vmId) });
    },
  });
}

export function useUpdateVMMapping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ vmId, mappingId, data }: { vmId: number; mappingId: number; data: Parameters<typeof updateVirtualModelMapping>[2] }) =>
      updateVirtualModelMapping(vmId, mappingId, data),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.mappings(vars.vmId) });
    },
  });
}

export function useDeleteVMMapping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ vmId, mappingId }: { vmId: number; mappingId: number }) =>
      deleteVirtualModelMapping(vmId, mappingId),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.mappings(vars.vmId) });
    },
  });
}

export function useBatchDeleteVMMappings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ vmId, ids }: { vmId: number; ids: number[] }) =>
      batchDeleteVirtualModelMapping(vmId, ids),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.mappings(vars.vmId) });
    },
  });
}

export function useBatchCreateVMMappings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ vmId, data }: { vmId: number; data: Parameters<typeof batchCreateVirtualModelMapping>[1] }) =>
      batchCreateVirtualModelMapping(vmId, data),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: virtualModelKeys.mappings(vars.vmId) });
    },
  });
}

export type { VirtualModel, VirtualModelMapping };
