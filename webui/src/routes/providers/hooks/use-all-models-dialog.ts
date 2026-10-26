import { useMemo, useState } from "react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import {
  getProviderModelCatalog,
  getProviderModels,
  getProviders,
  syncProviderModels,
  updateProvider,
  type Provider,
  type ProviderModelCatalog,
} from "@/lib/api";
import { providerKeys, useProviderModelCatalog } from "@/hooks/api/use-providers";
import { invalidateModelProviders } from "@/hooks/api/use-model-providers";
import { EMPTY_MODEL_CATALOG } from "@/lib/empty-constants";
import {
  buildConfigWithCustomModels,
  buildModelGroupMap,
  extractUniqueGroupsFromCatalog,
  matchModelGroupFilter,
  unionCatalogModels,
} from "@/lib/provider-models";
import type { Setter, Updater } from "@/stores/core/updater";
import type {
  AllModelsTypeFilter,
  ModelTestResult,
  UpstreamStatus,
} from "../types";
import { parseCustomModelsInput } from "../utils/config";
import {
  buildAutoActionsDescription,
  type AutoActionsFlags,
} from "../utils/auto-actions";

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
  const { data: catalogData = EMPTY_MODEL_CATALOG } = useProviderModelCatalog();
  const [allModelsList, setAllModelsList] = useState<string[]>([]);
  const [upstreamModelsList, setUpstreamModelsList] = useState<string[]>([]);
  const [upstreamStatus, setUpstreamStatus] =
    useState<UpstreamStatus>("disabled");
  const [allModelsGroupFilter, setAllModelsGroupFilter] = useState("all");

  const catalogEntry = useMemo(
    () => catalogData.find((item) => item.ProviderID === allModelsProvider?.ID),
    [catalogData, allModelsProvider?.ID],
  );

  // 标签与筛选统一：在目录 Upstream（分组白名单并集）中 → 上游；否则 → 自定义。
  // 若模型同时出现在 Custom 与 Upstream，按「上游」展示（与行内 badge 一致）。
  const upstreamSet = useMemo<Set<string>>(() => {
    return catalogEntry
      ? new Set(catalogEntry.Upstream.map((m) => m.ID.toLowerCase()))
      : new Set<string>();
  }, [catalogEntry]);

  const modelGroupMap = useMemo(() => buildModelGroupMap(catalogEntry), [catalogEntry]);
  const allModelsAvailableGroups = useMemo(
    () => extractUniqueGroupsFromCatalog(catalogEntry),
    [catalogEntry],
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
    return result.filter((model) =>
      matchModelGroupFilter(modelGroupMap.get(model), allModelsGroupFilter),
    );
  }, [allModelsList, allModelsSearchQuery, allModelsTypeFilter, upstreamSet, modelGroupMap, allModelsGroupFilter]);

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

  /** 弹窗是事件回调（非渲染期），目录初始化走 fetchQuery：fresh 缓存直接命中，
   *  否则打一次聚合端点。写/同步后统一 invalidate catalog 再走本入口回填。 */
  const fetchCatalogEntry = async (providerId: number): Promise<ProviderModelCatalog | null> => {
    const catalog = await queryClient.fetchQuery({
      queryKey: providerKeys.catalog(),
      queryFn: getProviderModelCatalog,
    });
    return catalog.find((item) => item.ProviderID === providerId) ?? null;
  };

  const openAllModelsDialog = async (provider: Provider) => {
    const entry = await fetchCatalogEntry(provider.ID);
    setAllModelsProvider(provider);
    setAllModelsList(entry ? unionCatalogModels(entry) : []);
    setSelectedAllModels([]);
    setCustomModelInput("");
    setAllModelsSearchQuery("");
    setAllModelsTypeFilter("all");
    setAllModelsGroupFilter("all");
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
    // 同步改写分组白名单 → 目录随之变化（fetchCatalogEntry 会重取最新值）
    await queryClient.invalidateQueries({ queryKey: providerKeys.catalog() });
    // 同步增删上游模型 → 后端在后台按开关改写关联（modelsync.ActionHooks），
    // 关联页是另一套缓存族，不一起失效就会在 staleTime 内继续显示陈旧行。
    invalidateModelProviders(queryClient);
    return updated;
  };

  /** 目录写路径唯一入口：只维护 config.custom_models（上游来源没有手动编辑入口——
   *  白名单由模型同步写入，供应商表单不带该字段）。legacy partial PUT：req.Config != ""
   *  时后端才写 config，故空对象 config 也不会误清。 */
  const persistModels = async (provider: Provider, customModels: string[]) => {
    const nextConfig = buildConfigWithCustomModels(provider.Config, customModels);
    await updateProvider(provider.ID, {
      name: provider.Name,
      type: provider.Type,
      config: nextConfig,
      console: provider.Console || "",
      proxy: provider.Proxy || "",
    });
    const updatedProvider = { ...provider, Config: nextConfig };
    patchProviderInLists(updatedProvider);
    // 目录缓存与 config 写不同源（聚合端点重算分组白名单），invalidate 让
    // upstreamSet / 三页目录随 custom 变化刷新
    await queryClient.invalidateQueries({ queryKey: providerKeys.catalog() });
    // custom_models 的增删走的是 PUT /providers/:id，后端随后按开关跑自动关联/自动清理，
    // 会改写 model_with_providers —— 与 sync 同一条缺口，关联页缓存一并失效。
    invalidateModelProviders(queryClient);
    return nextConfig;
  };

  const handleAddCustomModels = async (): Promise<boolean> => {
    if (!allModelsProvider) return false;
    const additions = parseCustomModelsInput(customModelInput);
    if (additions.length === 0) {
      toast.error("请先输入要添加的模型名称");
      return false;
    }
    const entry = await fetchCatalogEntry(allModelsProvider.ID);
    const upstreamLower = new Set(
      (entry?.Upstream ?? []).map((item) => item.ID.toLowerCase()),
    );
    const custom = entry?.Custom ?? [];
    // 已在上游（分组白名单）的模型不再写入 custom，避免列表 key 重复
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
      return false;
    }
    try {
      setAddingModels(true);
      await persistModels(allModelsProvider, merged);
      // 本地立即合并（invalidate 的 refetch 到达前保持弹窗即时反馈）
      setAllModelsList(
        unionCatalogModels({ Upstream: entry?.Upstream ?? [], Custom: merged }),
      );
      setCustomModelInput("");
      toast.success(`已添加 ${merged.length - custom.length} 个自定义模型`, {
        description: buildAutoActionsDescription(
          { associate: true },
          autoActionsFlags,
        ),
      });
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加自定义模型失败: ${message}`);
      console.error(err);
      return false;
    } finally {
      setAddingModels(false);
    }
  };

  const removeModelsFromAll = async (modelsToRemove: string[]) => {
    if (!allModelsProvider || modelsToRemove.length === 0) return;
    const entry = await fetchCatalogEntry(allModelsProvider.ID);
    const upstream = entry?.Upstream ?? [];
    const custom = entry?.Custom ?? [];
    const removalSet = new Set(
      modelsToRemove.map((item) => item.toLowerCase()),
    );
    // 只删 custom 行：上游来源（分组白名单）不可从目录删除，白名单由模型同步维护
    const nextCustom = custom.filter(
      (item) => !removalSet.has(item.toLowerCase()),
    );
    const removedCount = custom.length - nextCustom.length;
    if (removedCount === 0) {
      toast.info("没有可删除的模型");
      return;
    }

    try {
      setAddingModels(true);
      await persistModels(allModelsProvider, nextCustom);
      setAllModelsList(
        unionCatalogModels({ Upstream: upstream, Custom: nextCustom }),
      );
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
    // 同步改写的是分组白名单 → 目录在聚合端点侧重算；fetchQuery 拿回最新值
    const entry = await fetchCatalogEntry(provider.ID);
    setAllModelsList(entry ? unionCatalogModels(entry) : []);
    try {
      const upstreamModels = await getProviderModels(provider.ID);
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
    modelGroupMap,
    allModelsAvailableGroups,
    allModelsGroupFilter,
    setAllModelsGroupFilter,
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
