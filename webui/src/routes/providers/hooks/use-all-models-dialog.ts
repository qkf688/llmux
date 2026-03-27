import { useState } from "react";
import { toast } from "sonner";
import { getProviderModels, getProviders, syncProviderModels, updateProvider, type Provider } from "@/lib/api";
import { buildConfigWithModels, parseCustomModelsFromConfig, parseUpstreamModelsFromConfig } from "@/lib/provider-models";
import type { ModelTestResult, UpstreamStatus } from "../types";
import { extractAllModels, parseCustomModelsInput } from "../utils/config";
import { buildAutoActionsDescription, type AutoActionsFlags } from "../utils/auto-actions";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

type UseAllModelsDialogInput = {
  setProviders: Setter<Provider[]>;
  fetchProviders: () => Promise<void>;

  allModelsProvider: Provider | null;
  setAllModelsProvider: (provider: Provider | null) => void;
  setAllModelsOpen: (open: boolean) => void;

  selectedAllModels: string[];
  setSelectedAllModels: Setter<string[]>;

  customModelInput: string;
  setCustomModelInput: (value: string) => void;
  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;

  setAllModelsTestResults: (results: Updater<Record<string, ModelTestResult>>) => void;
  setAddingModels: (adding: boolean) => void;
  setSyncingModels: (syncing: boolean) => void;

  autoActionsFlags: AutoActionsFlags;
};

export function useAllModelsDialog({
  setProviders,
  fetchProviders,
  allModelsProvider,
  setAllModelsProvider,
  setAllModelsOpen,
  selectedAllModels,
  setSelectedAllModels,
  customModelInput,
  setCustomModelInput,
  allModelsSearchQuery,
  setAllModelsSearchQuery,
  setAllModelsTestResults,
  setAddingModels,
  setSyncingModels,
  autoActionsFlags,
}: UseAllModelsDialogInput) {
  const [allModelsList, setAllModelsList] = useState<string[]>([]);
  const [upstreamModelsList, setUpstreamModelsList] = useState<string[]>([]);
  const [upstreamStatus, setUpstreamStatus] = useState<UpstreamStatus>("disabled");

  const filteredAllModels =
    allModelsSearchQuery.trim() === ""
      ? allModelsList
      : allModelsList.filter((model) => model.toLowerCase().includes(allModelsSearchQuery.toLowerCase()));

  const isAllFilteredSelected = filteredAllModels.length > 0 && filteredAllModels.every((model) => selectedAllModels.includes(model));

  const toggleSelectAllModels = () => {
    if (filteredAllModels.length === 0) return;

    setSelectedAllModels((previous) => {
      const allSelected = filteredAllModels.every((model) => previous.includes(model));
      if (allSelected) {
        return previous.filter((model) => !filteredAllModels.includes(model));
      }
      return Array.from(new Set([...previous, ...filteredAllModels]));
    });
  };

  const openAllModelsDialog = async (provider: Provider) => {
    const allModels = extractAllModels(provider.Config);
    setAllModelsProvider(provider);
    setAllModelsList(allModels);
    setSelectedAllModels([]);
    setCustomModelInput("");
    setAllModelsSearchQuery("");
    setAllModelsTestResults({});
    setAllModelsOpen(true);
    setUpstreamModelsList([]);
    setUpstreamStatus("disabled");
  };

  const persistModels = async (provider: Provider, upstreamModels: string[], customModels: string[]) => {
    const nextConfig = buildConfigWithModels(provider.Config, upstreamModels, customModels);
    await updateProvider(provider.ID, {
      name: provider.Name,
      type: provider.Type,
      config: nextConfig,
      console: provider.Console || "",
      proxy: provider.Proxy || "",
    });
    setProviders((prev) => prev.map((item) => (item.ID === provider.ID ? { ...item, Config: nextConfig } : item)));
    return nextConfig;
  };

  const handleAddCustomModels = async () => {
    if (!allModelsProvider) return;
    const additions = parseCustomModelsInput(customModelInput);
    if (additions.length === 0) {
      toast.error("请先输入要添加的模型名称");
      return;
    }
    const upstream = parseUpstreamModelsFromConfig(allModelsProvider.Config);
    const custom = parseCustomModelsFromConfig(allModelsProvider.Config);
    const merged = Array.from(new Set([...custom, ...additions]));
    if (merged.length === custom.length) {
      toast.info("没有新的模型需要添加");
      return;
    }
    try {
      setAddingModels(true);
      const nextConfig = await persistModels(allModelsProvider, upstream, merged);
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList([...upstream, ...merged]);
      setCustomModelInput("");
      toast.success(`已添加 ${merged.length - custom.length} 个自定义模型`, {
        description: buildAutoActionsDescription({ associate: true }, autoActionsFlags),
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加自定义模型失败: ${message}`);
      console.error(err);
    } finally {
      setAddingModels(false);
    }
  };

  const removeModelsFromAll = async (modelsToRemove: string[]) => {
    if (!allModelsProvider || modelsToRemove.length === 0) return;
    const upstream = parseUpstreamModelsFromConfig(allModelsProvider.Config);
    const custom = parseCustomModelsFromConfig(allModelsProvider.Config);
    const removalSet = new Set(modelsToRemove.map((item) => item.toLowerCase()));
    const nextUpstream = upstream.filter((item) => !removalSet.has(item.toLowerCase()));
    const nextCustom = custom.filter((item) => !removalSet.has(item.toLowerCase()));
    const removedCount = upstream.length - nextUpstream.length + (custom.length - nextCustom.length);
    if (removedCount === 0) {
      toast.info("没有可删除的模型");
      return;
    }

    try {
      setAddingModels(true);
      const nextConfig = await persistModels(allModelsProvider, nextUpstream, nextCustom);
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList([...nextUpstream, ...nextCustom]);
      setSelectedAllModels([]);
      toast.success(`已移除 ${removedCount} 个模型`, {
        description: buildAutoActionsDescription({ clean: true }, autoActionsFlags),
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`移除模型失败: ${message}`);
      console.error(err);
    } finally {
      setAddingModels(false);
    }
  };

  const handleRemoveModelFromAll = async (modelId: string) => {
    await removeModelsFromAll([modelId]);
  };

  const handleRemoveSelectedModels = async () => {
    await removeModelsFromAll(selectedAllModels);
  };

  const handleSyncUpstreamModels = async () => {
    if (!allModelsProvider) return;

    try {
      setSyncingModels(true);
      setUpstreamStatus("loading");
      const result = await syncProviderModels(allModelsProvider.ID);

      if ("message" in result) {
        toast.info(result.message);
        try {
          const upstreamModels = await getProviderModels(allModelsProvider.ID, { source: "upstream" });
          const modelIds = upstreamModels.map((model) => model.id);
          setUpstreamModelsList(modelIds);
          setUpstreamStatus(modelIds.length > 0 ? "success" : "empty");
        } catch (err) {
          console.error("获取上游模型失败", err);
          setUpstreamModelsList([]);
          setUpstreamStatus("error");
        }
        return;
      }

      const { AddedCount, RemovedCount } = result;

      if (AddedCount > 0 || RemovedCount > 0) {
        toast.success(`同步完成：新增 ${AddedCount} 个，删除 ${RemovedCount} 个模型`, {
          description: buildAutoActionsDescription({
            associate: AddedCount > 0,
            clean: RemovedCount > 0,
          }, autoActionsFlags),
        });
      } else {
        toast.info("没有检测到模型变化");
      }

      await fetchProviders();

      const updatedProviders = await getProviders({});
      const updatedProvider = updatedProviders.find((provider) => provider.ID === allModelsProvider.ID);
      if (updatedProvider) {
        const updatedModels = extractAllModels(updatedProvider.Config);
        setAllModelsList(updatedModels);
        setAllModelsProvider(updatedProvider);

        try {
          const upstreamModels = await getProviderModels(updatedProvider.ID, { source: "upstream" });
          const modelIds = upstreamModels.map((model) => model.id);
          setUpstreamModelsList(modelIds);
          setUpstreamStatus(modelIds.length > 0 ? "success" : "empty");
        } catch (err) {
          console.error("获取上游模型失败", err);
          setUpstreamModelsList([]);
          setUpstreamStatus("error");
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`同步失败: ${message}`);
      console.error(err);
      setUpstreamStatus("error");
    } finally {
      setSyncingModels(false);
    }
  };

  return {
    allModelsList,
    filteredAllModels,
    isAllFilteredSelected,
    toggleSelectAllModels,
    setAllModelsList,
    upstreamModelsList,
    upstreamStatus,
    openAllModelsDialog,
    persistModels,
    handleAddCustomModels,
    handleRemoveModelFromAll,
    handleRemoveSelectedModels,
    handleSyncUpstreamModels,
  };
}
