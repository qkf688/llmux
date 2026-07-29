import { useState } from "react";
import { toast } from "sonner";
import { getProviderModels, type Provider, type ProviderModel } from "@/lib/api";
import { parseCustomModelsFromConfig, parseUpstreamModelsFromConfig } from "@/lib/provider-models";
import type { Setter } from "@/stores/core/updater";
import { buildAutoActionsDescription, type AutoActionsFlags } from "../utils/auto-actions";
import { getAllModelsForProvider } from "../utils/provider-models";

type UseUpstreamModelsDialogInput = {
  providers: Provider[];
  modelsOpenId: number | null;
  setModelsOpen: (open: boolean) => void;
  setModelsOpenId: (id: number | null) => void;

  modelsLoading: boolean;
  setModelsLoading: (loading: boolean) => void;
  addingModels: boolean;
  setAddingModels: (adding: boolean) => void;

  selectedUpstreamModels: string[];
  setSelectedUpstreamModels: Setter<string[]>;

  allModelsProvider: Provider | null;
  setAllModelsProvider: (provider: Provider | null) => void;
  setAllModelsList: Setter<string[]>;
  persistModels: (provider: Provider, upstreamModels: string[], customModels: string[]) => Promise<string>;

  autoActionsFlags: AutoActionsFlags;
};

export function useUpstreamModelsDialog({
  providers,
  modelsOpenId,
  setModelsOpen,
  setModelsOpenId,
  modelsLoading,
  setModelsLoading,
  addingModels,
  setAddingModels,
  selectedUpstreamModels,
  setSelectedUpstreamModels,
  allModelsProvider,
  setAllModelsProvider,
  setAllModelsList,
  persistModels,
  autoActionsFlags,
}: UseUpstreamModelsDialogInput) {
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([]);
  const [filteredProviderModels, setFilteredProviderModels] = useState<ProviderModel[]>([]);
  const [upstreamModelsCache, setUpstreamModelsCache] = useState<Record<number, ProviderModel[]>>({});

  const savedModelSet = new Set(
    getAllModelsForProvider(providers, modelsOpenId || 0).map((item) => item.toLowerCase()),
  );

  const selectableModelIds = filteredProviderModels
    .filter((model) => !savedModelSet.has(model.id.toLowerCase()))
    .map((model) => model.id);

  const isAllSelectableChecked =
    selectableModelIds.length > 0 && selectableModelIds.every((id) => selectedUpstreamModels.includes(id));

  const toggleSelectAll = () => {
    if (selectableModelIds.length === 0) {
      setSelectedUpstreamModels([]);
      return;
    }
    const hasUnselected = selectableModelIds.some((id) => !selectedUpstreamModels.includes(id));
    setSelectedUpstreamModels((prev) => {
      if (hasUnselected) {
        return Array.from(new Set([...prev, ...selectableModelIds]));
      }
      return prev.filter((id) => !selectableModelIds.includes(id));
    });
  };

  const fetchProviderModels = async (providerId: number, source: "upstream" | "all" = "upstream") => {
    try {
      setModelsLoading(true);
      const data = await getProviderModels(providerId, { source });
      const models = Array.isArray(data) ? data : [];
      setProviderModels(models);
      setFilteredProviderModels(models);
      if (source === "upstream") {
        setUpstreamModelsCache((prev) => ({ ...prev, [providerId]: models }));
      }
    } catch (err) {
      console.error("获取提供商模型失败", err);
      setProviderModels([]);
      setFilteredProviderModels([]);
    } finally {
      setModelsLoading(false);
    }
  };

  const openModelsDialog = async (providerId: number) => {
    setModelsOpen(true);
    setModelsOpenId(providerId);
    setSelectedUpstreamModels([]);

    const cached = upstreamModelsCache[providerId];
    if (cached && cached.length > 0) {
      setProviderModels(cached);
      setFilteredProviderModels(cached);
      return;
    }
    await fetchProviderModels(providerId, "upstream");
  };

  const refreshUpstreamModels = async () => {
    if (!modelsOpenId) return;
    setSelectedUpstreamModels([]);
    await fetchProviderModels(modelsOpenId, "upstream");
  };

  const handleUpstreamSearchChange = (value: string) => {
    const searchTerm = value.trim().toLowerCase();
    if (searchTerm === "") {
      setFilteredProviderModels(providerModels);
      return;
    }
    setFilteredProviderModels(providerModels.filter((model) => model.id.toLowerCase().includes(searchTerm)));
  };

  const handleAddUpstreamToAll = async () => {
    if (!modelsOpenId) return;
    const provider = providers.find((item) => item.ID === modelsOpenId);
    if (!provider) return;

    const upstream = parseUpstreamModelsFromConfig(provider.Config);
    const custom = parseCustomModelsFromConfig(provider.Config);
    const merged = Array.from(new Set([...upstream, ...selectedUpstreamModels]));
    if (merged.length === upstream.length) {
      toast.info("没有新的模型需要添加");
      return;
    }

    try {
      setAddingModels(true);
      const nextConfig = await persistModels(provider, merged, custom);
      if (allModelsProvider && allModelsProvider.ID === provider.ID) {
        setAllModelsProvider({ ...provider, Config: nextConfig });
        setAllModelsList([...merged, ...custom]);
      }
      setSelectedUpstreamModels([]);
      toast.success(`已添加 ${merged.length - upstream.length} 个模型到上游模型`, {
        description: buildAutoActionsDescription({ associate: true }, autoActionsFlags),
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加模型失败: ${message}`);
      console.error(err);
    } finally {
      setAddingModels(false);
    }
  };

  return {
    providerModels,
    filteredProviderModels,
    savedModelSet,
    selectableModelIds,
    isAllSelectableChecked,
    toggleSelectAll,
    modelsLoading,
    addingModels,
    openModelsDialog,
    refreshUpstreamModels,
    handleUpstreamSearchChange,
    handleAddUpstreamToAll,
  };
}
