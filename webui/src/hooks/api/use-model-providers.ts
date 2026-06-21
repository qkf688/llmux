import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getModelProviders, type ModelWithProvider } from '@/lib/api';

export const modelProviderKeys = {
  all: ['model-providers'] as const,
  lists: () => [...modelProviderKeys.all, 'list'] as const,
  list: (modelId: number | null) =>
    [...modelProviderKeys.lists(), modelId] as const,
};

export function useModelProvidersQuery(modelId: number | null) {
  return useQuery({
    queryKey: modelProviderKeys.list(modelId),
    queryFn: () => getModelProviders(modelId!),
    enabled: modelId !== null,
    select: (data) =>
      data.map((item) => ({
        ...item,
        CustomerHeaders: item.CustomerHeaders || {},
      })),
  });
}

export function invalidateModelProviders(queryClient: ReturnType<typeof useQueryClient>, modelId?: number | null) {
  if (modelId != null) {
    void queryClient.invalidateQueries({ queryKey: modelProviderKeys.list(modelId) });
  } else {
    void queryClient.invalidateQueries({ queryKey: modelProviderKeys.lists() });
  }
}

export type { ModelWithProvider };
