import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getLogs,
  deleteLog,
  batchDeleteLogs,
  clearAllLogs,
  clearFilteredLogs,
  getUserAgents,
  type ChatLog,
  type LogsResponse,
} from '@/lib/api';

export const logsKeys = {
  all: ['logs'] as const,
  lists: () => [...logsKeys.all, 'list'] as const,
  list: (page: number, pageSize: number, filters: Record<string, string | undefined>) =>
    [...logsKeys.lists(), { page, pageSize, ...filters }] as const,
  userAgents: () => [...logsKeys.all, 'user-agents'] as const,
};

export function useLogsQuery(page: number, pageSize: number, apiFilters: Record<string, string | undefined>) {
  return useQuery<LogsResponse>({
    queryKey: logsKeys.list(page, pageSize, apiFilters),
    queryFn: () => getLogs(page, pageSize, apiFilters),
  });
}

export function useUserAgents() {
  return useQuery({
    queryKey: logsKeys.userAgents(),
    queryFn: getUserAgents,
  });
}

export function useDeleteLog() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteLog,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: logsKeys.all });
    },
  });
}

export function useBatchDeleteLogs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: batchDeleteLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: logsKeys.all });
    },
  });
}

export function useClearAllLogs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearAllLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: logsKeys.all });
    },
  });
}

export function useClearFilteredLogs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearFilteredLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: logsKeys.all });
    },
  });
}

export type { ChatLog, LogsResponse };
