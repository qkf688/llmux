import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getProviders,
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
    },
  });
}

export function useClearProviderAssociations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearProviderAssociations,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
    },
  });
}

export function useSyncAllProviders() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: syncAllProviderModels,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
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

export type { Provider, ProviderTemplate, Settings, HealthCheckSettings };
