import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getModelSyncStats,
  getModelSyncLogs,
  getRecentAddedModels,
  syncAllProviderModels,
  deleteModelSyncLogs,
  clearModelSyncLogs,
  clearModelSyncErrorLogs,
  updateProvider,
  type ModelSyncStats,
  type ModelSyncLogsResponse,
  type RecentAddedModelsResponse,
} from '@/lib/api';
import { providerKeys } from './use-providers';

const LOG_PAGE_SIZE = 20;

export const modelSyncLogsKeys = {
  all: ['modelSyncLogs'] as const,
  stats: () => [...modelSyncLogsKeys.all, 'stats'] as const,
  logsList: () => [...modelSyncLogsKeys.all, 'logsList'] as const,
  logsListPage: (page: number, showUnchanged: boolean) =>
    [...modelSyncLogsKeys.logsList(), { page, pageSize: LOG_PAGE_SIZE, showUnchanged }] as const,
  recent: () => [...modelSyncLogsKeys.all, 'recent'] as const,
  errors: () => [...modelSyncLogsKeys.all, 'errors'] as const,
};

export function useModelSyncStatsQuery() {
  return useQuery<ModelSyncStats>({
    queryKey: modelSyncLogsKeys.stats(),
    queryFn: getModelSyncStats,
  });
}

export function useModelSyncLogsListQuery(
  page: number,
  showUnchanged: boolean,
  options?: { enabled?: boolean },
) {
  return useQuery<ModelSyncLogsResponse>({
    queryKey: modelSyncLogsKeys.logsListPage(page, showUnchanged),
    queryFn: () => getModelSyncLogs({ page, page_size: LOG_PAGE_SIZE, show_unchanged: showUnchanged }),
    enabled: options?.enabled,
  });
}

export function useRecentModelsQuery(options?: { enabled?: boolean }) {
  return useQuery<RecentAddedModelsResponse>({
    queryKey: modelSyncLogsKeys.recent(),
    queryFn: getRecentAddedModels,
    enabled: options?.enabled,
  });
}

export function useRecentErrorsQuery(options?: { enabled?: boolean }) {
  return useQuery<ModelSyncLogsResponse>({
    queryKey: modelSyncLogsKeys.errors(),
    queryFn: () => getModelSyncLogs({ page: 1, page_size: 100, status: 'error' }),
    enabled: options?.enabled,
  });
}

export function useSyncAllProvidersMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: syncAllProviderModels,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.all });
    },
  });
}

export function useDeleteModelSyncLogsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteModelSyncLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.all });
    },
  });
}

export function useClearModelSyncLogsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearModelSyncLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.all });
    },
  });
}

export function useClearModelSyncErrorLogsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearModelSyncErrorLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.all });
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.stats() });
    },
  });
}

export function useToggleProviderModelEndpointMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ providerId, enabled }: { providerId: number; enabled: boolean }) =>
      updateProvider(providerId, { model_endpoint: enabled }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: providerKeys.lists() });
      void qc.invalidateQueries({ queryKey: modelSyncLogsKeys.stats() });
    },
  });
}
