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
  autoImportPoolCredentials,
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

/** 「供应商分组 → 添加 Key」：建专属号池 + 导凭据一步完成（池 id 随响应返回）。 */
export function useAutoImportPoolCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; note?: string; keys: string[] }) =>
      autoImportPoolCredentials(body),
    onSuccess: (data) => {
      // 新池立刻并进列表缓存：否则号池列表要等 refetch 回来才认得它是专属池，
      // 这段窗口里「添加 Key」刚建好的池会被当成「不在列表里」→ 分组卡片显示成
      // 「尚未添加凭据」、切模式时还会被误解绑。缓存里的这一行只是临时占位，
      // 随后的 invalidatePoolList refetch 会用服务端真数据覆盖它。
      qc.setQueriesData<PoolListItem[]>({ queryKey: poolKeys.lists() }, (old) => {
        if (!old) return old;
        const now = new Date().toISOString();
        const created: PoolListItem = {
          ID: data.pool_id,
          Name: data.pool_name,
          Note: "",
          TestModel: "",
          AutoCreated: data.auto_created,
          CreatedAt: now,
          UpdatedAt: now,
          KeyCount: data.key_count,
          StatusCounts: {},
          ReferencedBy: 0,
        };
        return [...old.filter((p) => p.ID !== created.ID), created];
      });
      invalidatePoolList(qc);
      void qc.invalidateQueries({ queryKey: [...poolKeys.all, "credentials", data.pool_id] });
    },
  });
}

export type { PoolListItem, CredentialListData, BatchImportData };
