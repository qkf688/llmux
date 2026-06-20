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
  type Provider,
  type ProviderTemplate,
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
    onSuccess: () => {
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

export type { Provider, ProviderTemplate, Settings };
