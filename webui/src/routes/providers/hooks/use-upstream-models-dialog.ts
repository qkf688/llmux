import { useMemo, useState } from "react";
import { toast } from "sonner";
import { getProviderModels, type Provider, type ProviderModel } from "@/lib/api";
import { useProviderModelCatalog } from "@/hooks/api/use-providers";
import { EMPTY_MODEL_CATALOG } from "@/lib/empty-constants";
import { unionCatalogModels } from "@/lib/provider-models";
import type { Setter } from "@/stores/core/updater";
import { buildAutoActionsDescription, type AutoActionsFlags } from "../utils/auto-actions";

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
  setAllModelsList: Setter<string[]>;
  persistModels: (provider: Provider, customModels: string[]) => Promise<string>;

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
  setAllModelsList,
  persistModels,
  autoActionsFlags,
}: UseUpstreamModelsDialogInput) {
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([]);
  const [filteredProviderModels, setFilteredProviderModels] = useState<ProviderModel[]>([]);
  const [upstreamModelsCache, setUpstreamModelsCache] = useState<Record<number, ProviderModel[]>>({});
  const { data: catalogData = EMPTY_MODEL_CATALOG } = useProviderModelCatalog();

  // 去重基准 = 目录已收录模型（分组白名单并集 + custom），数据源与 all-models 弹窗同源
  const savedModelSet = useMemo(() => {
    const entry = catalogData.find((item) => item.ProviderID === modelsOpenId);
    return new Set(
      (entry ? unionCatalogModels(entry) : []).map((item) => item.toLowerCase()),
    );
  }, [catalogData, modelsOpenId]);

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

    // 落点 = custom：S5 后 config.upstream_models 是死键，上游模型入目录只能手填为自定义；
    // 上游来源的正式编辑入口在供应商表单分组白名单
    const entry = catalogData.find((item) => item.ProviderID === provider.ID);
    const custom = entry?.Custom ?? [];
    const additions = selectedUpstreamModels.filter(
      (model) => !savedModelSet.has(model.toLowerCase()),
    );
    if (additions.length === 0) {
      toast.info("没有新的模型需要添加");
      return;
    }

    try {
      setAddingModels(true);
      const merged = Array.from(new Set([...custom, ...additions]));
      await persistModels(provider, merged);
      if (allModelsProvider && allModelsProvider.ID === provider.ID) {
        setAllModelsList(
          unionCatalogModels({ Upstream: entry?.Upstream ?? [], Custom: merged }),
        );
      }
      setSelectedUpstreamModels([]);
      toast.success(`已添加 ${merged.length - custom.length} 个模型到目录（自定义）`, {
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
