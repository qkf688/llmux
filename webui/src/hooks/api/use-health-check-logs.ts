import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getHealthCheckLogs,
  clearHealthCheckLogs,
  type HealthCheckLogsResponse,
} from '@/lib/api';

export const healthCheckLogsKeys = {
  all: ['healthCheckLogs'] as const,
  lists: () => [...healthCheckLogsKeys.all, 'list'] as const,
  list: (page: number, pageSize: number, filters: Record<string, string | undefined>) =>
    [...healthCheckLogsKeys.lists(), { page, pageSize, ...filters }] as const,
};

export function useHealthCheckLogsQuery(
  page: number,
  pageSize: number,
  apiFilters: Record<string, string | undefined>,
) {
  return useQuery<HealthCheckLogsResponse>({
    queryKey: healthCheckLogsKeys.list(page, pageSize, apiFilters),
    queryFn: () => getHealthCheckLogs(page, pageSize, apiFilters),
  });
}

export function useClearHealthCheckLogs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearHealthCheckLogs,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: healthCheckLogsKeys.all });
    },
  });
}
