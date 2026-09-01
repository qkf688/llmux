import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  getPools,
  createPool,
  updatePool,
  deletePool,
  getPoolCredentials,
  createCredential,
  updateCredential,
  deleteCredential,
  batchUpdateCredentialStatus,
  batchDeleteCredentials,
  batchImportCredentials,
  type CredentialListParams,
  type PoolListItem,
  type CredentialListData,
  type BatchImportData,
} from "@/lib/api";

export const poolKeys = {
  all: ["pools"] as const,
  lists: () => [...poolKeys.all, "list"] as const,
  list: (filters?: { name?: string }) => [...poolKeys.lists(), filters ?? {}] as const,
  credentials: (poolId: number, params: CredentialListParams = {}) =>
    [...poolKeys.all, "credentials", poolId, params] as const,
};

export function usePools(filters?: { name?: string }) {
  return useQuery({
    queryKey: poolKeys.list(filters),
    queryFn: () => getPools(filters),
  });
}

export function usePoolCredentials(poolId: number | null, params: CredentialListParams = {}) {
  return useQuery({
    queryKey: poolKeys.credentials(poolId ?? 0, params),
    queryFn: () => getPoolCredentials(poolId!, params),
    enabled: poolId != null,
  });
}

function invalidatePoolList(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: poolKeys.lists() });
}

function invalidatePoolCredentials(
  qc: ReturnType<typeof useQueryClient>,
  poolId: number,
) {
  void qc.invalidateQueries({
    queryKey: [...poolKeys.all, "credentials", poolId],
  });
  invalidatePoolList(qc);
}

export function useCreatePool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createPool,
    onSuccess: () => invalidatePoolList(qc),
  });
}

export function useUpdatePool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Parameters<typeof updatePool>[1] }) =>
      updatePool(id, data),
    onSuccess: () => invalidatePoolList(qc),
  });
}

export function useDeletePool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deletePool,
    onSuccess: () => invalidatePoolList(qc),
  });
}

export function useCreateCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      poolId,
      data,
    }: {
      poolId: number;
      data: Parameters<typeof createCredential>[1];
    }) => createCredential(poolId, data),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useUpdateCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      poolId,
      credId,
      data,
    }: {
      poolId: number;
      credId: number;
      data: Parameters<typeof updateCredential>[2];
    }) => updateCredential(poolId, credId, data),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useDeleteCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ poolId, credId }: { poolId: number; credId: number }) =>
      deleteCredential(poolId, credId),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useBatchUpdateCredentialStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      poolId,
      ids,
      status,
    }: {
      poolId: number;
      ids: number[];
      status: string;
    }) => batchUpdateCredentialStatus(poolId, { ids, status }),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useBatchDeleteCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ poolId, ids }: { poolId: number; ids: number[] }) =>
      batchDeleteCredentials(poolId, { ids }),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useBatchImportCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ poolId, keys }: { poolId: number; keys: string[] }) =>
      batchImportCredentials(poolId, { keys }),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export type { PoolListItem, CredentialListData, BatchImportData };
