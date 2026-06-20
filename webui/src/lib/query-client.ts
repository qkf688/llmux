import { QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      refetchOnWindowFocus: false,
      staleTime: 60_000,
      gcTime: 5 * 60_000,
    },
    mutations: {
      onError: (error) => {
        const message = error instanceof Error ? error.message : '操作失败';
        toast.error(message);
      },
    },
  },
});
