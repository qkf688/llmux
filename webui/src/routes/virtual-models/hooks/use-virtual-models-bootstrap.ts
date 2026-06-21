import { useCallback, useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { toErrorMessage } from "@/lib/errors";
import { virtualModelKeys } from "@/hooks/api/use-virtual-models";
import { modelKeys } from "@/hooks/api/use-models";
import { providerKeys } from "@/hooks/api/use-providers";
import type { VirtualModelsPageState } from "@/stores/virtual-models";

type UseVirtualModelsBootstrapInput = {
  resetTransient: VirtualModelsPageState["resetTransient"];
};

export function useVirtualModelsBootstrap({
  resetTransient,
}: UseVirtualModelsBootstrapInput) {
  const queryClient = useQueryClient();

  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  const fetchInitialData = useCallback(async () => {
    try {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: virtualModelKeys.all }),
        queryClient.invalidateQueries({ queryKey: modelKeys.all }),
        queryClient.invalidateQueries({ queryKey: providerKeys.all }),
      ]);
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`获取数据失败: ${message}`);
      console.error(error);
    }
  }, [queryClient]);

  const refreshProvidersState = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: providerKeys.all });
  }, [queryClient]);

  const refreshMappings = useCallback(
    async (virtualModelId: number) => {
      await queryClient.invalidateQueries({ queryKey: virtualModelKeys.mappings(virtualModelId) });
    },
    [queryClient],
  );

  return {
    fetchInitialData,
    refreshProvidersState,
    refreshMappings,
  };
}
