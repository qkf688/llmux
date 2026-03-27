import { useCallback, useEffect } from "react";
import { toast } from "sonner";
import { getModels, getProviders, getVirtualModelMappings, getVirtualModels } from "@/lib/api";
import type { VirtualModelsPageState } from "@/stores/virtual-models";

const extractErrorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error));

type UseVirtualModelsBootstrapInput = {
  setLoading: VirtualModelsPageState["setLoading"];
  setVirtualModels: VirtualModelsPageState["setVirtualModels"];
  setRealModels: VirtualModelsPageState["setRealModels"];
  setProviders: VirtualModelsPageState["setProviders"];
  setBlacklistedProviders: VirtualModelsPageState["setBlacklistedProviders"];
  setMappings: VirtualModelsPageState["setMappings"];
  resetTransient: VirtualModelsPageState["resetTransient"];
};

export function useVirtualModelsBootstrap({
  setLoading,
  setVirtualModels,
  setRealModels,
  setProviders,
  setBlacklistedProviders,
  setMappings,
  resetTransient,
}: UseVirtualModelsBootstrapInput) {
  const fetchInitialData = useCallback(async () => {
    try {
      setLoading(true);
      const [virtualModelData, realModelData, providerData] = await Promise.all([
        getVirtualModels(),
        getModels(),
        getProviders(),
      ]);
      setVirtualModels(virtualModelData);
      setRealModels(realModelData);
      setProviders(providerData);
      setBlacklistedProviders(providerData.filter((provider) => provider.blacklisted));
    } catch (error) {
      const message = extractErrorMessage(error);
      toast.error(`获取数据失败: ${message}`);
      console.error(error);
    } finally {
      setLoading(false);
    }
  }, [setBlacklistedProviders, setLoading, setProviders, setRealModels, setVirtualModels]);

  useEffect(() => {
    void fetchInitialData();
    return () => {
      resetTransient();
    };
  }, [fetchInitialData, resetTransient]);

  const refreshProvidersState = useCallback(async () => {
    const latestProviders = await getProviders();
    setProviders(latestProviders);
    setBlacklistedProviders(latestProviders.filter((provider) => provider.blacklisted));
  }, [setBlacklistedProviders, setProviders]);

  const refreshMappings = useCallback(
    async (virtualModelId: number) => {
      const latestMappings = await getVirtualModelMappings(virtualModelId);
      setMappings(latestMappings);
    },
    [setMappings]
  );

  return {
    fetchInitialData,
    refreshProvidersState,
    refreshMappings,
  };
}

