import { QueryClient, QueryCache } from '@tanstack/react-query';
import { toast } from 'sonner';
import { toErrorMessage } from './errors';

export const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      toast.error(`加载失败: ${toErrorMessage(error)}`);
    },
  }),
  defaultOptions: {
    queries: {
      retry: 2,
      refetchOnWindowFocus: false,
      staleTime: 60_000,
      gcTime: 5 * 60_000,
    },
  },
});
