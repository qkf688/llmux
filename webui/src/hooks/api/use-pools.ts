import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  getPools,
  createPool,
  updatePool,
  deletePool,
  listCredentials,
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
    // 凭据是第一等资源：归属经 pool_id 过滤参数表达（见 lib/api/modules/catalog/credentials）。
    queryFn: () => listCredentials({ pool_id: poolId!, ...params }),
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
      data: { key: string; note?: string };
    }) => createCredential({ pool_id: poolId, ...data }),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useUpdateCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      credId,
      data,
    }: {
      poolId: number;
      credId: number;
      data: { note?: string | null; status?: string };
    }) => updateCredential(credId, data),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useDeleteCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ credId }: { poolId: number; credId: number }) => deleteCredential(credId),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useBatchUpdateCredentialStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      ids,
      status,
    }: {
      poolId: number;
      ids: number[];
      status: string;
    }) => batchUpdateCredentialStatus({ ids, status }),
    onSuccess: (_data, vars) => invalidatePoolCredentials(qc, vars.poolId),
  });
}

export function useBatchDeleteCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ ids }: { poolId: number; ids: number[] }) =>
      batchDeleteCredentials({ ids }),
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
