import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { invalidateModelProviders } from '@/hooks/api/use-model-providers';
import {
  getProviders,
  getProviderModelCatalog,
  getProviderTemplates,
  createProvider,
  updateProvider,
  deleteProvider,
  clearProviderAssociations,
  syncAllProviderModels,
  getSettings,
  updateSettings,
  getHealthCheckSettings,
  updateHealthCheckSettings,
  type Provider,
  type ProviderModelCatalog,
  type ProviderTemplate,
  type HealthCheckSettings,
} from '@/lib/api';
import type { Settings } from '@/lib/api';

export const providerKeys = {
  all: ['providers'] as const,
  lists: () => [...providerKeys.all, 'list'] as const,
  list: (filters?: { name?: string; type?: string }) =>
    [...providerKeys.lists(), filters ?? {}] as const,
  templates: () => [...providerKeys.all, 'templates'] as const,
  catalog: () => [...providerKeys.all, 'catalog'] as const,
};

export const settingsKeys = {
  all: ['settings'] as const,
};

export const healthCheckSettingsKeys = {
  all: ['healthCheckSettings'] as const,
};

export function useProviders(filters?: { name?: string; type?: string }) {
  return useQuery({
    queryKey: providerKeys.list(filters),
    queryFn: () => getProviders(filters),
  });
}

export function useProviderTemplates() {
  return useQuery({
    queryKey: providerKeys.templates(),
    queryFn: getProviderTemplates,
    staleTime: 5 * 60_000,
  });
}

/** 组织级模型目录（分组白名单并集 + custom）：models / model-providers /
 *  providers 三页的目录数据源。同步或写 custom 后 invalidate 本 key。 */
export function useProviderModelCatalog() {
  return useQuery({
    queryKey: providerKeys.catalog(),
    queryFn: getProviderModelCatalog,
  });
}

export function useSettings() {
  return useQuery<Settings>({
    queryKey: settingsKeys.all,
    queryFn: getSettings,
    staleTime: 5 * 60_000,
  });
}

export function useCreateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createProvider,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
    },
  });
}

export function useUpdateProvider(id: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof updateProvider>[1]) =>
      updateProvider(id, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
    },
  });
}

export function useDeleteProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteProvider,
    // 乐观更新：立即从缓存中移除被删项，让 AnimatePresence 稳定接管退场动画
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: providerKeys.lists() });
      const previous = qc.getQueriesData<Provider[]>({ queryKey: providerKeys.lists() });
      qc.setQueriesData<Provider[]>(
        { queryKey: providerKeys.lists() },
        (old) => old?.filter((p) => p.ID !== id),
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
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
      // 删除提供商后，其名下的关联在模型关联页同样会残留（同一 staleTime 窗口问题）
      invalidateModelProviders(qc);
    },
  });
}

export function useClearProviderAssociations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearProviderAssociations,
    // 清除的是 model-providers 表的行，影响面横跨两个缓存族：
    // 提供商页读 providerKeys，模型关联页读 modelProviderKeys（另一套 key）。
    // 只失效前者会让关联页在 staleTime（60s）内继续显示已删除的行——
    // 用户会以为"没清掉"并重复点击，第二次自然得到 deleted: 0。
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
      invalidateModelProviders(qc);
    },
  });
}

export function useSyncAllProviders() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: syncAllProviderModels,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
      // 同步改写分组白名单 → 组织级目录随之变化
      void qc.invalidateQueries({ queryKey: providerKeys.catalog() });
    },
  });
}

export function useHealthCheckSettingsQuery() {
  return useQuery<HealthCheckSettings>({
    queryKey: healthCheckSettingsKeys.all,
    queryFn: getHealthCheckSettings,
    staleTime: 5 * 60_000,
  });
}

export function useUpdateSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}

export function useUpdateHealthCheckSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateHealthCheckSettings,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: healthCheckSettingsKeys.all });
    },
  });
}

export type { Provider, ProviderModelCatalog, ProviderTemplate, Settings, HealthCheckSettings };
