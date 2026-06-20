import { useQueries } from '@tanstack/react-query';
import {
  getMetrics,
  getTotalMetrics,
  getDailyMetrics,
  getHourlyMetricsToday,
  getModelCounts,
  getRealModelCounts,
  getDatabaseStats,
  getProviderMetrics,
} from '@/lib/api';

export function useHomeMetrics() {
  return useQueries({
    queries: [
      {
        queryKey: ['metrics', 'today'],
        queryFn: () => getMetrics(0),
        staleTime: 60_000,
      },
      {
        queryKey: ['metrics', 'month'],
        queryFn: () => getMetrics(30),
        staleTime: 60_000,
      },
      {
        queryKey: ['metrics', 'all'],
        queryFn: getTotalMetrics,
        staleTime: 5 * 60_000,
      },
      {
        queryKey: ['metrics', 'daily'],
        queryFn: () => getDailyMetrics(380),
        staleTime: 60_000,
      },
      {
        queryKey: ['metrics', 'hourly'],
        queryFn: getHourlyMetricsToday,
        staleTime: 60_000,
      },
      {
        queryKey: ['models', 'real-counts'],
        queryFn: getRealModelCounts,
        staleTime: 60_000,
      },
      {
        queryKey: ['models', 'requested-counts'],
        queryFn: getModelCounts,
        staleTime: 60_000,
      },
      {
        queryKey: ['system', 'db-stats'],
        queryFn: getDatabaseStats,
        staleTime: 5 * 60_000,
      },
      {
        queryKey: ['providers', 'metrics'],
        queryFn: getProviderMetrics,
        staleTime: 60_000,
      },
    ],
  });
}
