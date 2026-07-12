import { useMemo, useState } from "react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import {
  getProviderModels,
  getProviders,
  syncProviderModels,
  updateProvider,
  type Provider,
} from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";
import {
  buildConfigWithModels,
  parseCustomModelsFromConfig,
  parseUpstreamModelsFromConfig,
} from "@/lib/provider-models";
import type { Updater } from "@/stores/core/updater";
import type {
  AllModelsTypeFilter,
  ModelTestResult,
  UpstreamStatus,
} from "../types";
import { extractAllModels, parseCustomModelsInput } from "../utils/config";
import {
  buildAutoActionsDescription,
  type AutoActionsFlags,
} from "../utils/auto-actions";

type Setter<T> = (value: Updater<T>) => void;

type UseAllModelsDialogInput = {
  allModelsProvider: Provider | null;
  setAllModelsProvider: (provider: Provider | null) => void;
  setAllModelsOpen: (open: boolean) => void;

  selectedAllModels: string[];
  setSelectedAllModels: Setter<string[]>;

  customModelInput: string;
  setCustomModelInput: (value: string) => void;
  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;

  allModelsTypeFilter: AllModelsTypeFilter;
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => void;

  setAllModelsTestResults: (
    results: Updater<Record<string, ModelTestResult>>,
  ) => void;
  setAddingModels: (adding: boolean) => void;
  setSyncingModels: (syncing: boolean) => void;

  autoActionsFlags: AutoActionsFlags;
};

export function useAllModelsDialog({
  allModelsProvider,
  setAllModelsProvider,
  setAllModelsOpen,
  selectedAllModels,
  setSelectedAllModels,
  customModelInput,
  setCustomModelInput,
  allModelsSearchQuery,
  setAllModelsSearchQuery,
  allModelsTypeFilter,
  setAllModelsTypeFilter,
  setAllModelsTestResults,
  setAddingModels,
  setSyncingModels,
  autoActionsFlags,
}: UseAllModelsDialogInput) {
  const queryClient = useQueryClient();
  const [allModelsList, setAllModelsList] = useState<string[]>([]);
  const [upstreamModelsList, setUpstreamModelsList] = useState<string[]>([]);
  const [upstreamStatus, setUpstreamStatus] =
    useState<UpstreamStatus>("disabled");

  const providerConfig = allModelsProvider?.Config;
  // 标签与筛选统一：在 upstream_models 中 → 上游；否则 → 自定义。
  // 若模型同时出现在 custom_models 与 upstream_models，按「上游」展示（与行内 badge 一致）。
  const upstreamSet = useMemo<Set<string>>(
    () =>
      providerConfig
        ? new Set(
            parseUpstreamModelsFromConfig(providerConfig).map((m) =>
              m.toLowerCase(),
            ),
          )
        : new Set<string>(),
    [providerConfig],
  );

  const filteredAllModels = useMemo<string[]>(() => {
    let result = allModelsList;
    if (allModelsSearchQuery.trim() !== "") {
      const q = allModelsSearchQuery.toLowerCase();
      result = result.filter((model) => model.toLowerCase().includes(q));
    }
    if (allModelsTypeFilter === "upstream") {
      result = result.filter((model) => upstreamSet.has(model.toLowerCase()));
    } else if (allModelsTypeFilter === "custom") {
      result = result.filter((model) => !upstreamSet.has(model.toLowerCase()));
    }
    return result;
  }, [allModelsList, allModelsSearchQuery, allModelsTypeFilter, upstreamSet]);

  const selectedSet = useMemo(
    () => new Set(selectedAllModels),
    [selectedAllModels],
  );
  const isAllFilteredSelected =
    filteredAllModels.length > 0 &&
    filteredAllModels.every((model) => selectedSet.has(model));

  const toggleSelectAllModels = () => {
    if (filteredAllModels.length === 0) return;

    setSelectedAllModels((previous) => {
      const previousSet = new Set(previous);
      const allSelected = filteredAllModels.every((model) =>
        previousSet.has(model),
      );
      if (allSelected) {
        const removeSet = new Set(filteredAllModels);
        return previous.filter((model) => !removeSet.has(model));
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
    setAllModelsTypeFilter("all");
    setAllModelsTestResults({});
    setAllModelsOpen(true);
    setUpstreamModelsList([]);
    setUpstreamStatus("disabled");
  };

  const patchProviderInLists = (provider: Provider) => {
    queryClient.setQueriesData<Provider[]>(
      { queryKey: providerKeys.lists() },
      (old) => old?.map((item) => (item.ID === provider.ID ? provider : item)),
    );
  };

  const refreshProviderFromServer = async (
    providerId: number,
  ): Promise<Provider | null> => {
    // 不走带 filters 的 list key，避免与当前筛选查询键不一致；结果写回所有 list 缓存
    const providers = await getProviders({});
    const updated =
      providers.find((provider) => provider.ID === providerId) ?? null;
    if (updated) {
      patchProviderInLists(updated);
    }
    await queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
    return updated;
  };

  const persistModels = async (
    provider: Provider,
    upstreamModels: string[],
    customModels: string[],
  ) => {
    const nextConfig = buildConfigWithModels(
      provider.Config,
      upstreamModels,
      customModels,
    );
    await updateProvider(provider.ID, {
      name: provider.Name,
      type: provider.Type,
      config: nextConfig,
      console: provider.Console || "",
      proxy: provider.Proxy || "",
    });
    const updatedProvider = { ...provider, Config: nextConfig };
    patchProviderInLists(updatedProvider);
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
    const upstreamLower = new Set(upstream.map((item) => item.toLowerCase()));
    // 已在上游的模型不再写入 custom，避免列表 key 重复与 config 双写
    const additionsNotInUpstream = additions.filter(
      (item) => !upstreamLower.has(item.toLowerCase()),
    );
    const merged = Array.from(new Set([...custom, ...additionsNotInUpstream]));
    if (merged.length === custom.length) {
      toast.info(
        additionsNotInUpstream.length === 0 && additions.length > 0
          ? "模型已在上游列表中"
          : "没有新的模型需要添加",
      );
      return;
    }
    try {
      setAddingModels(true);
      const nextConfig = await persistModels(
        allModelsProvider,
        upstream,
        merged,
      );
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList(extractAllModels(nextConfig));
      setCustomModelInput("");
      toast.success(`已添加 ${merged.length - custom.length} 个自定义模型`, {
        description: buildAutoActionsDescription(
          { associate: true },
          autoActionsFlags,
        ),
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
    const removalSet = new Set(
      modelsToRemove.map((item) => item.toLowerCase()),
    );
    const nextUpstream = upstream.filter(
      (item) => !removalSet.has(item.toLowerCase()),
    );
    const nextCustom = custom.filter(
      (item) => !removalSet.has(item.toLowerCase()),
    );
    const removedCount =
      upstream.length -
      nextUpstream.length +
      (custom.length - nextCustom.length);
    if (removedCount === 0) {
      toast.info("没有可删除的模型");
      return;
    }

    try {
      setAddingModels(true);
      const nextConfig = await persistModels(
        allModelsProvider,
        nextUpstream,
        nextCustom,
      );
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList(extractAllModels(nextConfig));
      setSelectedAllModels([]);
      toast.success(`已移除 ${removedCount} 个模型`, {
        description: buildAutoActionsDescription(
          { clean: true },
          autoActionsFlags,
        ),
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

  const applyProviderModelsToDialog = async (provider: Provider) => {
    setAllModelsProvider(provider);
    setAllModelsList(extractAllModels(provider.Config));
    try {
      const upstreamModels = await getProviderModels(provider.ID, {
        source: "upstream",
      });
      const modelIds = upstreamModels.map((model) => model.id);
      setUpstreamModelsList(modelIds);
      setUpstreamStatus(modelIds.length > 0 ? "success" : "empty");
    } catch (err) {
      console.error("获取上游模型失败", err);
      setUpstreamModelsList([]);
      setUpstreamStatus("error");
    }
  };

  const handleSyncUpstreamModels = async () => {
    if (!allModelsProvider) return;

    try {
      setSyncingModels(true);
      setUpstreamStatus("loading");
      const result = await syncProviderModels(allModelsProvider.ID);

      if ("message" in result) {
        toast.info(result.message);
        const updatedProvider = await refreshProviderFromServer(
          allModelsProvider.ID,
        );
        if (updatedProvider) {
          await applyProviderModelsToDialog(updatedProvider);
        } else {
          setUpstreamStatus("error");
        }
        return;
      }

      const { AddedCount, RemovedCount } = result;

      if (AddedCount > 0 || RemovedCount > 0) {
        toast.success(
          `同步完成：新增 ${AddedCount} 个，删除 ${RemovedCount} 个模型`,
          {
            description: buildAutoActionsDescription(
              {
                associate: AddedCount > 0,
                clean: RemovedCount > 0,
              },
              autoActionsFlags,
            ),
          },
        );
      } else {
        toast.info("没有检测到模型变化");
      }

      const updatedProvider = await refreshProviderFromServer(
        allModelsProvider.ID,
      );
      if (updatedProvider) {
        await applyProviderModelsToDialog(updatedProvider);
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
    upstreamSet,
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
