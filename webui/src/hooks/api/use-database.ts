import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getDatabaseStats, vacuumDatabase, type DatabaseStats } from '@/lib/api';

export const databaseKeys = {
  all: ['database'] as const,
  stats: () => [...databaseKeys.all, 'stats'] as const,
};

export function useDatabaseStats() {
  return useQuery<DatabaseStats>({
    queryKey: databaseKeys.stats(),
    queryFn: getDatabaseStats,
  });
}

export function useVacuumDatabase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: vacuumDatabase,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: databaseKeys.stats() });
    },
  });
}

export type { DatabaseStats };
