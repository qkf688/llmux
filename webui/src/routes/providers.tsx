import { useState, useEffect } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import Loading from "@/components/loading";
import { Label } from "@/components/ui/label";
import {
  getProviders,
  getSettings,
  createProvider,
  updateProvider,
  deleteProvider,
  getProviderTemplates,
  getProviderModels,
  syncProviderModels,
  syncAllProviderModels,
  testProviderModel
} from "@/lib/api";
import type { Provider, ProviderTemplate, ProviderModel } from "@/lib/api";
import { buildConfigWithModels, parseAllModelsFromConfig, parseUpstreamModelsFromConfig, parseCustomModelsFromConfig } from "@/lib/provider-models";
import { toast } from "sonner";
import { Spinner } from "@/components/ui/spinner";

// 定义表单验证模式
const formSchema = z.object({
  name: z.string().min(1, { message: "提供商名称不能为空" }),
  type: z.string().min(1, { message: "提供商类型不能为空" }),
  base_url: z.string().min(1, { message: "Base URL 不能为空" }),
  api_key: z.string().min(1, { message: "API Key 不能为空" }),
  // Anthropic 特有字段
  beta: z.string().optional(),
  version: z.string().optional(),
  console: z.string().optional(),
  custom_models: z.string().optional(),
  proxy: z.string().optional(),
  model_endpoint: z.boolean().optional(),
});

// 将表单字段转换为 JSON 配置字符串
function buildConfigFromForm(values: z.infer<typeof formSchema>): string {
  const customModels = parseCustomModelsInput(values.custom_models);
  const baseConfig: Record<string, unknown> = {
    base_url: values.base_url,
    api_key: values.api_key,
  };

  if (values.type === "anthropic") {
    baseConfig.beta = values.beta || "";
    baseConfig.version = values.version || "2023-06-01";
  }

  if (customModels.length > 0) {
    baseConfig.custom_models = customModels;
  }

  return JSON.stringify(baseConfig);
}

// 从 JSON 配置字符串解析为表单字段
function parseConfigToForm(config: string, _type?: string): {
  base_url: string;
  api_key: string;
  beta?: string;
  version?: string;
  custom_models: string[];
} {
  try {
    const parsed = JSON.parse(config);
    return {
      base_url: parsed.base_url || "",
      api_key: parsed.api_key || "",
      beta: parsed.beta || "",
      version: parsed.version || "",
      custom_models: Array.isArray(parsed.custom_models)
        ? parsed.custom_models.filter((item: unknown) => typeof item === "string" && item.trim() !== "")
        : [],
    };
  } catch {
    return {
      base_url: "",
      api_key: "",
      beta: "",
      version: "",
      custom_models: [],
    };
  }
}

function parseCustomModelsInput(input?: string): string[] {
  if (!input) {
    return [];
  }
  return input
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

const extractAllModels = (config: string) => parseAllModelsFromConfig(config);

export default function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [providerTemplates, setProviderTemplates] = useState<ProviderTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [modelsOpen, setModelsOpen] = useState(false);
  const [modelsOpenId, setModelsOpenId] = useState<number | null>(null);
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([]);
  const [filteredProviderModels, setFilteredProviderModels] = useState<ProviderModel[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [selectedUpstreamModels, setSelectedUpstreamModels] = useState<string[]>([]);
  const [upstreamModelsCache, setUpstreamModelsCache] = useState<Record<number, ProviderModel[]>>({});
  const [allModelsOpen, setAllModelsOpen] = useState(false);
  const [allModelsProvider, setAllModelsProvider] = useState<Provider | null>(null);
  const [allModelsList, setAllModelsList] = useState<string[]>([]);
  const [selectedAllModels, setSelectedAllModels] = useState<string[]>([]);
  const [customModelInput, setCustomModelInput] = useState("");
  const [allModelsSearchQuery, setAllModelsSearchQuery] = useState("");
  const [allModelsTestResults, setAllModelsTestResults] = useState<Record<string, { loading: boolean; success: boolean | null; error?: string }>>({});
  const [addingModels, setAddingModels] = useState(false);
  const [batchTesting, setBatchTesting] = useState(false);
  const [batchTestProgress, setBatchTestProgress] = useState({ total: 0, completed: 0, success: 0, failed: 0, testing: 0 });
  const [testAbortController, setTestAbortController] = useState<AbortController | null>(null);
  const [showApiKey, setShowApiKey] = useState(false);
  const [syncingModels, setSyncingModels] = useState(false);
  const [syncingAll, setSyncingAll] = useState(false);
  const [upstreamModelsList, setUpstreamModelsList] = useState<string[]>([]);
  const [upstreamStatus, setUpstreamStatus] = useState<'loading' | 'success' | 'empty' | 'error' | 'disabled'>('disabled');
  const [autoAssociateOnAddEnabled, setAutoAssociateOnAddEnabled] = useState(false);
  const [autoCleanOnDeleteEnabled, setAutoCleanOnDeleteEnabled] = useState(false);

  // 筛选条件
  const [nameFilter, setNameFilter] = useState<string>("");
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [availableTypes, setAvailableTypes] = useState<string[]>([]);

  // 初始化表单
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      type: "",
      base_url: "",
      api_key: "",
      beta: "",
      version: "",
      console: "",
      custom_models: "",
      proxy: "",
      model_endpoint: true,
    },
  });

  // 监听类型变化，用于显示/隐藏 Anthropic 特有字段
  const watchedType = form.watch("type");

  useEffect(() => {
    fetchProviders();
    fetchProviderTemplates();
    fetchSettings();
  }, []);

  // 监听筛选条件变化
  useEffect(() => {
    fetchProviders();
  }, [nameFilter, typeFilter]);

  useEffect(() => {
    if (!modelsOpen) {
      setSelectedUpstreamModels([]);
    }
  }, [modelsOpen]);

  useEffect(() => {
    if (!allModelsOpen) {
      setSelectedAllModels([]);
      setAllModelsTestResults({});
    }
  }, [allModelsOpen]);

  const fetchProviders = async () => {
    try {
      setLoading(true);
      // 处理筛选条件，"all"表示不过滤，空字符串表示不过滤
      const name = nameFilter.trim() || undefined;
      const type = typeFilter === "all" ? undefined : typeFilter;

      const data = await getProviders({ name, type });
      setProviders(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商列表失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchProviderTemplates = async () => {
    try {
      const data = await getProviderTemplates();
      setProviderTemplates(data);
      // 设置可用的提供商类型
      const types = data.map(template => template.type);
      setAvailableTypes(types);
    } catch (err) {
      console.error("获取提供商模板失败", err);
    }
  };

  const fetchSettings = async () => {
    try {
      const data = await getSettings();
      setAutoAssociateOnAddEnabled(data.auto_associate_on_add ?? false);
      setAutoCleanOnDeleteEnabled(data.auto_clean_on_delete ?? false);
    } catch (err) {
      console.error("获取系统设置失败", err);
      setAutoAssociateOnAddEnabled(false);
      setAutoCleanOnDeleteEnabled(false);
    }
  };

  const buildAutoActionsDescription = (options: { associate?: boolean; clean?: boolean }) => {
    const actions: string[] = [];
    if (options.associate && autoAssociateOnAddEnabled) {
      actions.push("自动关联");
    }
    if (options.clean && autoCleanOnDeleteEnabled) {
      actions.push("自动清理无效关联");
    }
    if (actions.length === 0) return undefined;
    return `已触发${actions.join("、")}（后台异步）`;
  };

  const fetchProviderModels = async (providerId: number, source: "upstream" | "all" = "upstream") => {
    try {
      setModelsLoading(true);
      const data = await getProviderModels(providerId, { source });
      // 确保 data 是数组，防止后端返回 null 导致白屏
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

  const getAllModelsForProvider = (providerId: number): string[] => {
    const provider = providers.find((item) => item.ID === providerId);
    if (!provider) return [];
    return extractAllModels(provider.Config);
  };

  const toggleSelectAllModels = () => {
    if (filteredAllModels.length === 0) return;
    if (selectedAllModels.length >= filteredAllModels.length && filteredAllModels.every(m => selectedAllModels.includes(m))) {
      // 如果当前选中的包含所有过滤后的模型，则取消选中这些
      setSelectedAllModels(selectedAllModels.filter(m => !filteredAllModels.includes(m)));
    } else {
      // 否则选中所有过滤后的模型
      setSelectedAllModels(Array.from(new Set([...selectedAllModels, ...filteredAllModels])));
    }
  };

  const persistModels = async (provider: Provider, upstreamModels: string[], customModels: string[]) => {
    const nextConfig = buildConfigWithModels(provider.Config, upstreamModels, customModels);
    await updateProvider(provider.ID, {
      name: provider.Name,
      type: provider.Type,
      config: nextConfig,
      console: provider.Console || "",
      proxy: provider.Proxy || ""
    });
    setProviders((prev) =>
      prev.map((item) => item.ID === provider.ID ? { ...item, Config: nextConfig } : item)
    );
    return nextConfig;
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
        description: buildAutoActionsDescription({ associate: true }),
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加模型失败: ${message}`);
      console.error(err);
    } finally {
      setAddingModels(false);
    }
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
        description: buildAutoActionsDescription({ associate: true }),
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
    const removedCount = (upstream.length - nextUpstream.length) + (custom.length - nextCustom.length);
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
        description: buildAutoActionsDescription({ clean: true }),
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

  const handleToggleModelEndpoint = async (provider: Provider) => {
    const newValue = !(provider.ModelEndpoint ?? true);
    try {
      await updateProvider(provider.ID, {
        name: provider.Name,
        type: provider.Type,
        config: provider.Config,
        console: provider.Console || "",
        proxy: provider.Proxy || "",
        model_endpoint: newValue
      });
      setProviders((prev) =>
        prev.map((item) => item.ID === provider.ID ? { ...item, ModelEndpoint: newValue } : item)
      );
      toast.success(`已${newValue ? "启用" : "禁用"}模型端点`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新模型端点失败: ${message}`);
      console.error(err);
    }
  };

  const refreshUpstreamModels = async () => {
    if (!modelsOpenId) return;
    setSelectedUpstreamModels([]);
    await fetchProviderModels(modelsOpenId, "upstream");
  };

  const copyModelName = async (modelName: string) => {
    await navigator.clipboard.writeText(modelName);
    toast.success(`已复制模型名称: ${modelName}`);
  };

  const handleTestAllModel = async (modelName: string) => {
    if (!allModelsProvider) return;
    setAllModelsTestResults((prev) => ({
      ...prev,
      [modelName]: { loading: true, success: null }
    }));
    try {
      await testProviderModel(allModelsProvider.ID, modelName);
      setAllModelsTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: true }
      }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setAllModelsTestResults((prev) => ({
        ...prev,
        [modelName]: { loading: false, success: false, error: message }
      }));
    }
  };

  // 选择所有测试成功的模型
  const selectAllSuccessful = () => {
    const successfulModels = Object.entries(allModelsTestResults)
      .filter(([_, result]) => result.success === true)
      .map(([modelName]) => modelName);
    
    if (successfulModels.length === 0) {
      toast.info("当前没有测试成功的模型");
      return;
    }
    
    setSelectedAllModels(successfulModels);
    toast.success(`已选择 ${successfulModels.length} 个测试成功的模型`);
  };

  // 选择所有测试失败的模型
  const selectAllFailed = () => {
    const failedModels = Object.entries(allModelsTestResults)
      .filter(([_, result]) => result.success === false)
      .map(([modelName]) => modelName);
    
    if (failedModels.length === 0) {
      toast.info("当前没有测试失败的模型");
      return;
    }
    
    setSelectedAllModels(failedModels);
    toast.success(`已选择 ${failedModels.length} 个测试失败的模型`);
  };

  // 批量测试核心逻辑
  const testSingleModelInBatch = async (
    model: string,
    testing: Set<string>,
    signal: AbortSignal
  ) => {
    if (signal.aborted) {
      testing.delete(model);
      return;
    }

    try {
      await handleTestAllModel(model);
      
      setBatchTestProgress(prev => ({
        ...prev,
        completed: prev.completed + 1,
        success: prev.success + 1,
        testing: testing.size - 1
      }));
    } catch (error) {
      setBatchTestProgress(prev => ({
        ...prev,
        completed: prev.completed + 1,
        failed: prev.failed + 1,
        testing: testing.size - 1
      }));
    } finally {
      testing.delete(model);
    }
  };

  const startBatchTest = async (models: string[]) => {
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }

    // 初始化状态
    setBatchTesting(true);
    setBatchTestProgress({
      total: models.length,
      completed: 0,
      success: 0,
      failed: 0,
      testing: 0
    });

    const abortController = new AbortController();
    setTestAbortController(abortController);

    // 并发控制
    const concurrency = 3;
    const queue = [...models];
    const testing = new Set<string>();

    try {
      while (queue.length > 0 && !abortController.signal.aborted) {
        // 控制并发数
        while (testing.size < concurrency && queue.length > 0) {
          const model = queue.shift()!;
          testing.add(model);
          
          setBatchTestProgress(prev => ({
            ...prev,
            testing: testing.size
          }));

          // 异步测试
          testSingleModelInBatch(model, testing, abortController.signal);
        }

        // 等待至少一个完成
        await new Promise(resolve => setTimeout(resolve, 100));
      }

      // 等待所有测试完成
      while (testing.size > 0 && !abortController.signal.aborted) {
        await new Promise(resolve => setTimeout(resolve, 100));
      }

      // 显示完成通知
      if (!abortController.signal.aborted) {
        const { success, failed } = batchTestProgress;
        toast.success(
          `批量测试完成：成功 ${success} 个，失败 ${failed} 个`,
          { duration: 5000 }
        );
      }
    } finally {
      setBatchTesting(false);
      setTestAbortController(null);
    }
  };

  const handleBatchTestAll = async () => {
    const models = filteredAllModels;
    if (models.length === 0) {
      toast.error("没有可测试的模型");
      return;
    }
    await startBatchTest(models);
  };

  const handleBatchTestSelected = async () => {
    if (selectedAllModels.length === 0) {
      toast.error("请先选择要测试的模型");
      return;
    }
    await startBatchTest(selectedAllModels);
  };

  const handleCancelBatchTest = () => {
    if (testAbortController) {
      testAbortController.abort();
      toast.info("已取消批量测试");
    }
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
    setUpstreamStatus('disabled');
  };

  const handleSyncUpstreamModels = async () => {
    if (!allModelsProvider) return;

    try {
      setSyncingModels(true);
      setUpstreamStatus('loading');
      const result = await syncProviderModels(allModelsProvider.ID);

      if ('message' in result) {
        toast.info(result.message);
        // 重新获取上游模型状态
        try {
          const upstreamModels = await getProviderModels(allModelsProvider.ID, { source: "upstream" });
          const modelIds = upstreamModels.map(m => m.id);
          setUpstreamModelsList(modelIds);
          setUpstreamStatus(modelIds.length > 0 ? 'success' : 'empty');
        } catch (err) {
          console.error("获取上游模型失败", err);
          setUpstreamModelsList([]);
          setUpstreamStatus('error');
        }
        return;
      }

      const { AddedCount, RemovedCount } = result;

      if (AddedCount > 0 || RemovedCount > 0) {
        toast.success(`同步完成：新增 ${AddedCount} 个，删除 ${RemovedCount} 个模型`, {
          description: buildAutoActionsDescription({
            associate: AddedCount > 0,
            clean: RemovedCount > 0,
          }),
        });
      } else {
        toast.info("没有检测到模型变化");
      }

      // 刷新提供商列表
      await fetchProviders();

      // 更新当前弹窗的模型列表
      const updatedProviders = await getProviders({});
      const updatedProvider = updatedProviders.find(p => p.ID === allModelsProvider.ID);
      if (updatedProvider) {
        const updatedModels = extractAllModels(updatedProvider.Config);
        setAllModelsList(updatedModels);
        setAllModelsProvider(updatedProvider);

        // 重新获取上游模型列表
        try {
          const upstreamModels = await getProviderModels(updatedProvider.ID, { source: "upstream" });
          const modelIds = upstreamModels.map(m => m.id);
          setUpstreamModelsList(modelIds);
          setUpstreamStatus(modelIds.length > 0 ? 'success' : 'empty');
        } catch (err) {
          console.error("获取上游模型失败", err);
          setUpstreamModelsList([]);
          setUpstreamStatus('error');
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`同步失败: ${message}`);
      console.error(err);
      setUpstreamStatus('error');
    } finally {
      setSyncingModels(false);
    }
  };

  const handleSyncAllProviders = async () => {
    try {
      setSyncingAll(true);
      const result = await syncAllProviderModels();
      const addedTotal = typeof (result as any).added_total === "number" ? (result as any).added_total : 0;
      const removedTotal = typeof (result as any).removed_total === "number" ? (result as any).removed_total : 0;
      const syncedProviders = typeof (result as any).synced_providers === "number"
        ? (result as any).synced_providers
        : Array.isArray((result as any).logs) ? (result as any).logs.length : 0;

      if (Array.isArray((result as any).logs) && (result as any).logs.length > 0) {
        toast.success(`同步完成：新增 ${addedTotal} 个，删除 ${removedTotal} 个模型`, {
          description: syncedProviders > 0 ? `涉及 ${syncedProviders} 个提供商` : undefined,
        });
      } else {
        toast.info((result as any).message ?? "没有检测到模型变化");
      }

      await fetchProviders();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`同步失败: ${message}`);
      console.error(err);
    } finally {
      setSyncingAll(false);
    }
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    try {
      const config = buildConfigFromForm(values);
      await createProvider({
        name: values.name,
        type: values.type,
        config: config,
        console: values.console || "",
        proxy: values.proxy || "",
        model_endpoint: values.model_endpoint ?? true
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 创建成功`);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "", model_endpoint: true });
      fetchProviders();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建提供商失败: ${message}`);
      console.error(err);
    }
  };

  const handleUpdate = async (values: z.infer<typeof formSchema>) => {
    if (!editingProvider) return;
    try {
      const config = buildConfigFromForm(values);
      await updateProvider(editingProvider.ID, {
        name: values.name,
        type: values.type,
        config: config,
        console: values.console || "",
        proxy: values.proxy || "",
        model_endpoint: values.model_endpoint
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 更新成功`);
      setEditingProvider(null);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "", model_endpoint: true });
      fetchProviders();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新提供商失败: ${message}`);
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      const targetProvider = providers.find((provider) => provider.ID === deleteId);
      await deleteProvider(deleteId);
      setDeleteId(null);
      fetchProviders();
      const message = `提供商 ${targetProvider?.Name ?? deleteId} 删除成功`;
      if (autoCleanOnDeleteEnabled) {
        toast.success(message, { description: "已触发自动清理无效关联（后台异步）" });
      } else {
        toast.success(message);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除提供商失败: ${message}`);
      console.error(err);
    }
  };

  const openEditDialog = (provider: Provider) => {
    setEditingProvider(provider);
    setShowApiKey(false);
    const configFields = parseConfigToForm(provider.Config, provider.Type);
    form.reset({
      name: provider.Name,
      type: provider.Type,
      base_url: configFields.base_url,
      api_key: configFields.api_key,
      beta: configFields.beta || "",
      version: configFields.version || "",
      console: provider.Console || "",
      custom_models: configFields.custom_models.join("\n"),
      proxy: provider.Proxy || "",
      model_endpoint: provider.ModelEndpoint ?? true,
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    setShowApiKey(false);
    form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "", model_endpoint: true });
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const hasFilter = nameFilter.trim() !== "" || typeFilter !== "all";

  const savedModelSet = new Set(getAllModelsForProvider(modelsOpenId || 0).map((item) => item.toLowerCase()));
  const selectableModelIds = filteredProviderModels
    .filter((model) => !savedModelSet.has(model.id.toLowerCase()))
    .map((model) => model.id);
  const isAllSelectableChecked = selectableModelIds.length > 0 && selectableModelIds.every((id) => selectedUpstreamModels.includes(id));

  // 过滤全部模型列表
  const filteredAllModels = allModelsSearchQuery.trim() === ""
    ? allModelsList
    : allModelsList.filter(model => model.toLowerCase().includes(allModelsSearchQuery.toLowerCase()));

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

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">提供商管理</h2>
          </div>
          <div className="flex w-full sm:w-auto items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleSyncAllProviders}
              disabled={syncingAll}
              className="h-9"
            >
              {syncingAll ? "同步中..." : "一键同步上游模型"}
            </Button>
          </div>
        </div>
      </div>
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:gap-4">
          <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商名称</Label>
            <Input
              placeholder="输入名称"
              value={nameFilter}
              onChange={(e) => setNameFilter(e.target.value)}
              className="h-8 w-full text-xs px-2"
            />
          </div>
          <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">类型</Label>
            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="选择类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {availableTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-end col-span-2 sm:col-span-1 sm:justify-end">
            <Button onClick={openCreateDialog} className="h-8 w-full text-xs sm:w-auto sm:ml-auto">
              添加提供商
            </Button>
          </div>
        </div>
      </div>
      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载提供商列表" />
          </div>
        ) : providers.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm text-center px-6">
            {hasFilter ? '未找到匹配的提供商' : '暂无提供商数据'}
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="hidden sm:block w-full overflow-x-auto">
              <Table className="min-w-[1100px]">
                <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>名称</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>全部模型</TableHead>
                    <TableHead>模型端点</TableHead>
                    <TableHead className="w-[260px]">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {providers.map((provider) => {
                    const allModels = extractAllModels(provider.Config);
                    return (
                      <TableRow key={provider.ID}>
                        <TableCell className="font-mono text-xs text-muted-foreground">{provider.ID}</TableCell>
                        <TableCell className="font-medium">{provider.Name}</TableCell>
                        <TableCell className="text-sm">{provider.Type}</TableCell>
                        <TableCell>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => openAllModelsDialog(provider)}
                            className="gap-1.5"
                          >
                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
                              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                            </svg>
                            {allModels.length}
                          </Button>
                        </TableCell>
                        <TableCell>
                          <Switch
                            checked={provider.ModelEndpoint ?? true}
                            onCheckedChange={() => handleToggleModelEndpoint(provider)}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => openEditDialog(provider)}>
                              编辑
                            </Button>
                            <Button variant="secondary" size="sm" onClick={() => openModelsDialog(provider.ID)}>
                              获取模型
                            </Button>
                            <AlertDialog>
                              <AlertDialogTrigger asChild>
                                <Button variant="destructive" size="sm" onClick={() => openDeleteDialog(provider.ID)}>
                                  删除
                                </Button>
                              </AlertDialogTrigger>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    此操作无法撤销。这将永久删除该提供商。
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                                  <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
            <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-3 divide-y divide-border">
              {providers.map((provider) => {
                const allModels = extractAllModels(provider.Config);
                return (
                  <div key={provider.ID} className="py-3 space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <h3 className="font-semibold text-sm truncate">{provider.Name}</h3>
                        <p className="text-[11px] text-muted-foreground">ID: {provider.ID}</p>
                        <p className="text-[11px] text-muted-foreground">类型: {provider.Type || "未知"}</p>
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 px-2 text-xs mt-1 gap-1.5"
                          onClick={() => openAllModelsDialog(provider)}
                        >
                          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-3.5 w-3.5">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                          </svg>
                          {allModels.length}
                        </Button>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-[11px] text-muted-foreground">模型端点:</span>
                          <Switch
                            checked={provider.ModelEndpoint ?? true}
                            onCheckedChange={() => handleToggleModelEndpoint(provider)}
                          />
                        </div>
                      </div>
                      <div className="flex flex-wrap justify-end gap-1.5">
                        <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => openEditDialog(provider)}>
                          编辑
                        </Button>
                        <Button variant="secondary" size="sm" className="h-7 px-2 text-xs" onClick={() => openModelsDialog(provider.ID)}>
                          模型
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="destructive" size="sm" className="h-7 px-2 text-xs" onClick={() => openDeleteDialog(provider.ID)}>
                              删除
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
                              <AlertDialogDescription>
                                此操作无法撤销。这将永久删除该提供商。
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                              <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingProvider ? "编辑提供商" : "添加提供商"}
            </DialogTitle>
            <DialogDescription>
              {editingProvider
                ? "修改提供商信息"
                : "添加一个新的提供商"}
            </DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(editingProvider ? handleUpdate : handleCreate)} className="space-y-4 min-w-0">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="type"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>类型</FormLabel>
                    <FormControl>
                      <select
                        {...field}
                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                        onChange={(e) => {
                          field.onChange(e);
                          // When type changes, populate config fields with template defaults if available
                          const selectedTemplate = providerTemplates.find(t => t.type === e.target.value);
                          if (selectedTemplate) {
                            const parsed = parseConfigToForm(selectedTemplate.template, e.target.value);
                            form.setValue("base_url", parsed.base_url);
                            // Don't set api_key as it should be entered by user
                            if (e.target.value === "anthropic") {
                              form.setValue("version", parsed.version || "2023-06-01");
                              form.setValue("beta", parsed.beta || "");
                            }
                          }
                        }}
                      >
                        <option value="">请选择提供商类型</option>
                        {providerTemplates.map((template) => (
                          <option key={template.type} value={template.type}>
                            {template.type}
                          </option>
                        ))}
                      </select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="base_url"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Base URL</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="https://api.openai.com/v1" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="api_key"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>API Key</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Input
                          {...field}
                          type={showApiKey ? "text" : "password"}
                          placeholder="sk-..."
                          className="pr-10"
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="absolute right-1.5 top-1/2 -translate-y-1/2 h-8 w-8"
                          onClick={() => setShowApiKey((prev) => !prev)}
                          aria-label={showApiKey ? "隐藏 API Key" : "显示 API Key"}
                        >
                          {showApiKey ? (
                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
                              <path strokeLinecap="round" strokeLinejoin="round" d="M3 3l18 18M9.88 9.88A3 3 0 0114.12 14.12M10.73 5.08A9.53 9.53 0 0112 5c5 0 9 4.5 9 7s-4 7-9 7a9.53 9.53 0 01-1.27-.08M6.61 6.61C4.13 8.2 3 10 3 12c0 2.5 4 7 9 7a9.35 9.35 0 003.39-.64" />
                            </svg>
                          ) : (
                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
                              <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.477 0 8.268 2.943 9.542 7-1.274 4.057-5.065 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                              <circle cx="12" cy="12" r="3" />
                            </svg>
                          )}
                        </Button>
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />


              {/* Anthropic 特有字段 */}
              {watchedType === "anthropic" && (
                <>
                  <FormField
                    control={form.control}
                    name="version"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Version</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder="2023-06-01" />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="beta"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Beta（可选）</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder="可选的 beta 标识" />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </>
              )}

              <FormField
                control={form.control}
                name="console"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>控制台地址（可选）</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="https://example.com/console" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="proxy"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>代理地址（可选）</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="http://user:pass@host:port" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="model_endpoint"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border p-3">
                    <div className="space-y-0.5">
                      <FormLabel>模型端点</FormLabel>
                      <div className="text-sm text-muted-foreground">
                        是否支持从上游获取模型列表
                      </div>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                  取消
                </Button>
                <Button type="submit">
                  {editingProvider ? "更新" : "创建"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 全部模型对话框 */}
      <Dialog open={allModelsOpen} onOpenChange={setAllModelsOpen}>
        <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
          <DialogHeader className="flex-shrink-0">
            <DialogTitle>{allModelsProvider?.Name || "当前提供商"}的全部模型</DialogTitle>
            <DialogDescription>
              手动维护模型缓存，可添加自定义模型或批量删除不再需要的条目。
            </DialogDescription>
          </DialogHeader>

          {/* 上游模型状态提示 */}
          {upstreamStatus === 'loading' && (
            <div className="flex items-center gap-2 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md text-sm text-blue-800">
              <svg className="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              正在获取上游模型...
            </div>
          )}
          {upstreamStatus === 'success' && (
            <div className="flex items-center gap-2 px-3 py-2 bg-green-50 border border-green-200 rounded-md text-sm text-green-800">
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
              已获取 {upstreamModelsList.length} 个上游模型
            </div>
          )}
          {upstreamStatus === 'empty' && (
            <div className="flex items-center gap-2 px-3 py-2 bg-yellow-50 border border-yellow-200 rounded-md text-sm text-yellow-800">
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              上游未返回任何模型
            </div>
          )}
          {upstreamStatus === 'error' && (
            <div className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-md text-sm text-red-800">
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
              获取上游模型失败
            </div>
          )}

          <div className="flex flex-col gap-4 flex-1 min-h-0">
            <div className="flex flex-col gap-2 flex-1 min-h-0">
              <div className="flex flex-col gap-2 flex-shrink-0">
                {/* 第一行：标题 + 数量 + 搜索框 */}
                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-semibold whitespace-nowrap">模型列表</p>
                    <span className="text-xs text-muted-foreground whitespace-nowrap">
                      {allModelsSearchQuery
                        ? `匹配 ${filteredAllModels.length} / ${allModelsList.length}`
                        : `${allModelsList.length} 个`}
                    </span>
                  </div>
                  <Input
                    placeholder="搜索模型名称..."
                    value={allModelsSearchQuery}
                    onChange={(e) => setAllModelsSearchQuery(e.target.value)}
                    className="h-8 flex-1 min-w-0"
                  />
                </div>
                
                {/* 第二行：测试结果统计（条件渲染）*/}
                {Object.keys(allModelsTestResults).length > 0 && (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>已测试: {Object.keys(allModelsTestResults).length}</span>
                    <span className="text-muted-foreground">|</span>
                    <span className="text-green-600">
                      成功: {Object.values(allModelsTestResults).filter(r => r.success === true).length}
                    </span>
                    <span className="text-muted-foreground">|</span>
                    <span className="text-red-600">
                      失败: {Object.values(allModelsTestResults).filter(r => r.success === false).length}
                    </span>
                  </div>
                )}

                {/* 批量测试进度条 */}
                {batchTesting && (
                  <div className="flex items-center gap-3 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md">
                    <div className="flex-1">
                      <div className="flex items-center justify-between text-xs text-blue-800 mb-1">
                        <span>
                          测试进度：{batchTestProgress.completed}/{batchTestProgress.total}
                          (成功: {batchTestProgress.success}, 失败: {batchTestProgress.failed}, 进行中: {batchTestProgress.testing})
                        </span>
                        <span>{Math.round((batchTestProgress.completed / batchTestProgress.total) * 100)}%</span>
                      </div>
                      <div className="w-full bg-blue-200 rounded-full h-2">
                        <div
                          className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${(batchTestProgress.completed / batchTestProgress.total) * 100}%` }}
                        />
                      </div>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleCancelBatchTest}
                      className="h-7 text-xs"
                    >
                      取消
                    </Button>
                  </div>
                )}

                <div className="flex items-center gap-1 flex-wrap">
                  {(allModelsProvider?.ModelEndpoint ?? true) && (
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant="secondary"
                            size="icon"
                            className="h-8 w-8"
                            onClick={handleSyncUpstreamModels}
                            disabled={syncingModels || batchTesting}
                          >
                            {syncingModels ? (
                              <Spinner className="h-4 w-4" />
                            ) : (
                              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                              </svg>
                            )}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{syncingModels ? "同步中..." : "同步上游模型"}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  )}
                  
                  {/* 批量测试按钮 */}
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="default"
                          size="icon"
                          className="h-8 w-8"
                          onClick={handleBatchTestAll}
                          disabled={filteredAllModels.length === 0 || batchTesting || addingModels}
                        >
                          {batchTesting ? (
                            <Spinner className="h-4 w-4" />
                          ) : (
                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                              <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                          )}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>批量测试所有模型</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>

                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="secondary"
                          size="icon"
                          className="h-8 w-8"
                          onClick={handleBatchTestSelected}
                          disabled={selectedAllModels.length === 0 || batchTesting || addingModels}
                        >
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                          </svg>
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>批量测试选中的 {selectedAllModels.length} 个模型</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  
                  {/* 选择成功和失败按钮 */}
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={selectAllSuccessful}
                          disabled={Object.values(allModelsTestResults).filter(r => r.success === true).length === 0 || batchTesting}
                        >
                          <svg className="h-4 w-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                          </svg>
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>选择测试成功的模型</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>

                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={selectAllFailed}
                          disabled={Object.values(allModelsTestResults).filter(r => r.success === false).length === 0 || batchTesting}
                        >
                          <svg className="h-4 w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                          </svg>
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>选择测试失败的模型</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={toggleSelectAllModels}
                          disabled={filteredAllModels.length === 0 || batchTesting}
                        >
                          {filteredAllModels.length > 0 && filteredAllModels.every(m => selectedAllModels.includes(m)) ? (
                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          ) : (
                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                          )}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>
                        {filteredAllModels.length > 0 && filteredAllModels.every(m => selectedAllModels.includes(m)) ? "取消全选" : "全选"}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="destructive"
                          size="icon"
                          className="h-8 w-8"
                          onClick={handleRemoveSelectedModels}
                          disabled={selectedAllModels.length === 0 || addingModels || batchTesting}
                        >
                          {addingModels ? (
                            <Spinner className="h-4 w-4" />
                          ) : (
                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                            </svg>
                          )}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>
                        {addingModels ? "删除中..." : `删除所选${selectedAllModels.length > 0 ? `（${selectedAllModels.length}）` : ""}`}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <span
                    className={`text-xs text-muted-foreground ml-1 inline-flex min-w-[64px] justify-end tabular-nums ${
                      selectedAllModels.length > 0 ? "" : "invisible"
                    }`}
                  >
                    已选 {selectedAllModels.length} 个
                  </span>
                </div>
              </div>
              <div className="border rounded-md flex-1 min-h-0 overflow-y-auto">
                {allModelsList.length === 0 ? (
                  <div className="text-sm text-muted-foreground text-center py-4">暂无缓存模型</div>
                ) : filteredAllModels.length === 0 ? (
                  <div className="text-sm text-muted-foreground text-center py-4">没有找到匹配的模型</div>
                ) : (
                  filteredAllModels.map((model) => {
                    const checked = selectedAllModels.includes(model);
                    const upstreamModels = allModelsProvider ? parseUpstreamModelsFromConfig(allModelsProvider.Config) : [];
                    const isUpstream = upstreamModels.includes(model);
                    const testResult = allModelsTestResults[model];
                    return (
                      <div
                        key={model}
                        className={`flex items-center justify-between px-3 py-2.5 text-sm gap-2 transition-colors border-b last:border-b-0 ${
                          checked ? "bg-blue-50/80" : "hover:bg-muted/50"
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <Checkbox
                            checked={checked}
                            onCheckedChange={(value) => {
                              if (value) {
                                setSelectedAllModels((prev) => Array.from(new Set([...prev, model])));
                              } else {
                                setSelectedAllModels((prev) => prev.filter((item) => item !== model));
                              }
                            }}
                            aria-label={`选择模型 ${model}`}
                          />
                          <div className="flex items-center gap-1.5 min-w-0">
                            <span className="truncate font-mono text-xs">{model}</span>
                            <span className={`flex-shrink-0 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${
                              isUpstream
                                ? "bg-blue-100 text-blue-700"
                                : "bg-gray-100 text-gray-600"
                            }`}>
                              {isUpstream ? "上游" : "自定义"}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-1 flex-shrink-0">
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7"
                                  onClick={() => handleTestAllModel(model)}
                                  disabled={!!testResult?.loading || batchTesting}
                                >
                                  {testResult?.loading ? (
                                    <Spinner className="h-3.5 w-3.5" />
                                  ) : testResult?.success === true ? (
                                    <svg className="h-3.5 w-3.5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                      <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                                    </svg>
                                  ) : testResult?.success === false ? (
                                    <svg className="h-3.5 w-3.5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                  ) : (
                                    <svg className="h-3.5 w-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                  )}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>
                                {testResult?.loading ? "测试中..." :
                                 testResult?.success === true ? "测试成功" :
                                 testResult?.success === false ? testResult.error || "测试失败" :
                                 "测试模型可用性"}
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>

                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7"
                                  onClick={() => copyModelName(model)}
                                >
                                  <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" />
                                  </svg>
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>复制名称</TooltipContent>
                            </Tooltip>
                          </TooltipProvider>

                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7 text-muted-foreground hover:text-destructive"
                                  onClick={() => handleRemoveModelFromAll(model)}
                                  disabled={addingModels || batchTesting}
                                >
                                  <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                  </svg>
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>移除</TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
              <div className="flex items-center justify-between gap-3 flex-shrink-0">
                <Textarea
                  value={customModelInput}
                  onChange={(e) => setCustomModelInput(e.target.value)}
                  placeholder="每行一个模型 ID，可用来自定义或补充上游未返回的模型"
                  className="h-16 resize-none flex-1"
                />
                <div className="flex gap-2">
                  <Button size="sm" onClick={handleAddCustomModels} disabled={addingModels || !allModelsProvider || batchTesting}>
                    {addingModels ? "提交中..." : "添加"}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setAllModelsOpen(false)}>
                    关闭
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* 模型列表对话框 */}
      <Dialog open={modelsOpen} onOpenChange={setModelsOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{providers.find(v => v.ID === modelsOpenId)?.Name} 上游模型</DialogTitle>
            <DialogDescription>
              从上游拉取的模型列表，勾选后可加入“全部模型”缓存。
            </DialogDescription>
          </DialogHeader>

          <div className="flex items-center justify-between gap-2 mb-3">
            <div className="text-sm text-muted-foreground">
              {modelsLoading
                ? "正在从上游获取..."
                : `上游返回 ${providerModels.length}  ${getAllModelsForProvider(modelsOpenId || 0).length} 个`}
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={toggleSelectAll}
                disabled={selectableModelIds.length === 0}
              >
                {isAllSelectableChecked ? "取消全选" : "全选可添加"}
              </Button>
              <Button variant="outline" size="sm" onClick={refreshUpstreamModels} disabled={!modelsOpenId || modelsLoading}>
                {modelsLoading ? "刷新中" : "刷新上游"}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleAddUpstreamToAll}
                disabled={selectedUpstreamModels.length === 0 || addingModels}
              >
                {addingModels ? "同步中..." : `添加到全部模型${selectedUpstreamModels.length > 0 ? `（${selectedUpstreamModels.length}）` : ""}`}
              </Button>
            </div>
          </div>

          {!modelsLoading && providerModels.length > 0 && (
            <div className="mb-3">
              <Input
                placeholder="搜索模型 ID"
                onChange={(e) => {
                  const searchTerm = e.target.value.toLowerCase();
                  if (searchTerm === '') {
                    setFilteredProviderModels(providerModels);
                  } else {
                    const filteredModels = providerModels.filter(model =>
                      model.id.toLowerCase().includes(searchTerm)
                    );
                    setFilteredProviderModels(filteredModels);
                  }
                }}
                className="w-full"
              />
            </div>
          )}

          {modelsLoading ? (
            <Loading message="加载模型列表" />
          ) : (
            <div className="max-h-96 overflow-y-auto space-y-2">
              {filteredProviderModels.length === 0 ? (
                <div className="text-center text-gray-500 py-8">
                  {providerModels.length === 0 ? '暂无模型数据' : '未找到匹配的模型'}
                </div>
              ) : (
                (() => {
                  return filteredProviderModels.map((model) => {
                    const isSaved = savedModelSet.has(model.id.toLowerCase());
                    const checked = selectedUpstreamModels.includes(model.id);
                    return (
                      <div
                        key={model.id}
                        className={`flex items-center justify-between p-2 border rounded-lg ${
                          isSaved ? "border-gray-300 bg-gray-50/50" : "border-border bg-background"
                        }`}
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <Checkbox
                            checked={checked}
                            disabled={isSaved}
                            onCheckedChange={(value) => {
                              if (value) {
                                setSelectedUpstreamModels((prev) => Array.from(new Set([...prev, model.id])));
                              } else {
                                setSelectedUpstreamModels((prev) => prev.filter((item) => item !== model.id));
                              }
                            }}
                          />
                          <div className="min-w-0">
                            <div className={`font-medium truncate ${isSaved ? "text-muted-foreground/70" : ""}`}>
                              {model.id}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => copyModelName(model.id)}
                                  className="min-w-12"
                                >
                                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" aria-hidden="true" className="h-4 w-4"><path strokeLinecap="round" strokeLinejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path></svg>
                                </Button>
                              </TooltipTrigger>
                            </Tooltip>
                          </TooltipProvider>
                        </div>
                      </div>
                    );
                  });
                })()
              )}
            </div>
          )}

          <DialogFooter>
            <Button onClick={() => setModelsOpen(false)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
 
