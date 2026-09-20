/**
 * API Keys 数据层 hook。
 *
 * 明文生命周期说明（#5 定案的残余风险）：列表端点恒回明文（方案 §2 复制模型），
 * 因此全部 key 明文必然进入 react-query 内存缓存（全站默认 staleTime 60s /
 * gcTime 5min；后端 no-store 只挡 HTTP/磁盘层）。这与号池「列表回掩码、明文
 * 只按需走 raw 端点、不进缓存」的模型不同——数据源形态决定，本域接受该破例。
 * 会话失效清缓存已收口（Stage 9）：`authStore.clearSession` 统一 `queryClient.clear()`，
 * 登出 / 401 失效后缓存不再残留明文。
 */
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listApiKeys,
  createApiKey,
  updateApiKey,
  deleteApiKey,
  type ApiKeyCreateInput,
  type ApiKeyUpdateInput,
} from "@/lib/api";

export const apiKeyKeys = {
  all: ["api-keys"] as const,
  lists: () => [...apiKeyKeys.all, "list"] as const,
};

export function useApiKeys() {
  return useQuery({
    queryKey: apiKeyKeys.lists(),
    queryFn: () => listApiKeys(),
  });
}

function invalidateApiKeyList(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: apiKeyKeys.lists() });
}

export function useCreateApiKey() {
  const qc = useQueryClient();
  return useMutation({
    // 箭头包装而非简写：防 react-query 把内部 context 作第二参传进 API 函数，
    // 保证 lib/api 契约面上只收声明过的入参
    mutationFn: (values: ApiKeyCreateInput) => createApiKey(values),
    onSuccess: () => invalidateApiKeyList(qc),
  });
}

export function useUpdateApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: ApiKeyUpdateInput }) => updateApiKey(id, data),
    onSuccess: () => invalidateApiKeyList(qc),
  });
}

export function useDeleteApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => deleteApiKey(id),
    onSuccess: () => invalidateApiKeyList(qc),
  });
}
