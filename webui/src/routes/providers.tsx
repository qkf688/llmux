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
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
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
  createProvider,
  updateProvider,
  deleteProvider,
  getProviderTemplates,
  getProviderModels
} from "@/lib/api";
import type { Provider, ProviderTemplate, ProviderModel } from "@/lib/api";
import { buildConfigWithAllModels, parseAllModelsFromConfig } from "@/lib/provider-models";
import { toast } from "sonner";

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
  const [addingModels, setAddingModels] = useState(false);
  const [showApiKey, setShowApiKey] = useState(false);

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
    },
  });

  // 监听类型变化，用于显示/隐藏 Anthropic 特有字段
  const watchedType = form.watch("type");

  useEffect(() => {
    fetchProviders();
    fetchProviderTemplates();
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
    if (allModelsList.length === 0) return;
    if (selectedAllModels.length === allModelsList.length) {
      setSelectedAllModels([]);
    } else {
      setSelectedAllModels(allModelsList);
    }
  };

  const persistAllModels = async (provider: Provider, models: string[]) => {
    const nextConfig = buildConfigWithAllModels(provider.Config, models);
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

    const existing = extractAllModels(provider.Config);
    const merged = Array.from(new Set([...existing, ...selectedUpstreamModels]));
    if (merged.length === existing.length) {
      toast.info("没有新的模型需要添加");
      return;
    }

    try {
      setAddingModels(true);
      const nextConfig = await persistAllModels(provider, merged);
      if (allModelsProvider && allModelsProvider.ID === provider.ID) {
        setAllModelsProvider({ ...provider, Config: nextConfig });
        setAllModelsList(merged);
      }
      setSelectedUpstreamModels([]);
      toast.success(`已添加 ${merged.length - existing.length} 个模型到全部模型`);
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
    const existing = extractAllModels(allModelsProvider.Config);
    const merged = Array.from(new Set([...existing, ...additions]));
    if (merged.length === existing.length) {
      toast.info("没有新的模型需要添加");
      return;
    }
    try {
      setAddingModels(true);
      const nextConfig = await persistAllModels(allModelsProvider, merged);
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList(merged);
      setCustomModelInput("");
      toast.success(`已添加 ${merged.length - existing.length} 个自定义模型`);
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
    const existing = extractAllModels(allModelsProvider.Config);
    const removalSet = new Set(modelsToRemove.map((item) => item.toLowerCase()));
    const next = existing.filter((item) => !removalSet.has(item.toLowerCase()));
    const removedCount = existing.length - next.length;
    if (removedCount === 0) {
      toast.info("没有可删除的模型");
      return;
    }

    try {
      setAddingModels(true);
      const nextConfig = await persistAllModels(allModelsProvider, next);
      const updatedProvider = { ...allModelsProvider, Config: nextConfig };
      setAllModelsProvider(updatedProvider);
      setAllModelsList(next);
      setSelectedAllModels([]);
      toast.success(`已从全部模型移除 ${removedCount} 个模型`);
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

  const refreshUpstreamModels = async () => {
    if (!modelsOpenId) return;
    setSelectedUpstreamModels([]);
    await fetchProviderModels(modelsOpenId, "upstream");
  };

  const copyModelName = async (modelName: string) => {
    await navigator.clipboard.writeText(modelName);
    toast.success(`已复制模型名称: ${modelName}`);
  };

  const openAllModelsDialog = (provider: Provider) => {
    const allModels = extractAllModels(provider.Config);
    setAllModelsProvider(provider);
    setAllModelsList(allModels);
    setSelectedAllModels([]);
    setCustomModelInput("");
    setAllModelsOpen(true);
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    try {
      const config = buildConfigFromForm(values);
      await createProvider({
        name: values.name,
        type: values.type,
        config: config,
        console: values.console || "",
        proxy: values.proxy || ""
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 创建成功`);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "" });
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
        proxy: values.proxy || ""
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 更新成功`);
      setEditingProvider(null);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "" });
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
      toast.success(`提供商 ${targetProvider?.Name ?? deleteId} 删除成功`);
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
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    setShowApiKey(false);
    form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "", proxy: "" });
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
                    <TableHead>控制台</TableHead>
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
                          {provider.Console ? (
                            <Button
                              title={provider.Console}
                              variant="outline"
                              size="sm"
                              onClick={() => window.open(provider.Console, '_blank')}
                            >
                              前往
                            </Button>
                          ) : (
                            <span className="text-muted-foreground">暂未设置</span>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => openEditDialog(provider)}>
                              编辑
                            </Button>
                            <Button variant="secondary" size="sm" onClick={() => openModelsDialog(provider.ID)}>
                              模型列表
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
                        {provider.Console && (
                          <Button
                            variant="link"
                            className="px-0 h-auto text-[11px]"
                            onClick={() => window.open(provider.Console, '_blank')}
                          >
                            控制台
                          </Button>
                        )}
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

              <FormField
                control={form.control}
                name="custom_models"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>全部模型（可选，本地缓存）</FormLabel>
                    <FormControl>
                      <Textarea {...field} placeholder="每行一个模型 ID，优先使用此列表，无需从上游重复获取" className="h-28" />
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
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{allModelsProvider?.Name || "当前提供商"}的全部模型</DialogTitle>
            <DialogDescription>
              手动维护模型缓存，可添加自定义模型或批量删除不再需要的条目。
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold">全部模型列表</p>
                  <span className="text-xs text-muted-foreground">已缓存 {allModelsList.length} 个</span>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={toggleSelectAllModels}
                    disabled={allModelsList.length === 0}
                  >
                    {selectedAllModels.length === allModelsList.length && allModelsList.length > 0 ? "取消全选" : "全选"}
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={handleRemoveSelectedModels}
                    disabled={selectedAllModels.length === 0 || addingModels}
                  >
                    {addingModels ? "删除中..." : `删除所选${selectedAllModels.length > 0 ? `（${selectedAllModels.length}）` : ""}`}
                  </Button>
                </div>
              </div>
              <div className="border rounded-md max-h-60 overflow-y-auto divide-y">
                {allModelsList.length === 0 ? (
                  <div className="text-sm text-muted-foreground text-center py-4">暂无缓存模型</div>
                ) : (
                  allModelsList.map((model) => {
                    const checked = selectedAllModels.includes(model);
                    return (
                      <div key={model} className="flex items-center justify-between px-3 py-2 text-sm gap-3">
                        <div className="flex items-center gap-3 min-w-0">
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
                          <span className="truncate">{model}</span>
                        </div>
                        <div className="flex items-center gap-2 flex-shrink-0">
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 px-2"
                            onClick={() => copyModelName(model)}
                          >
                            复制
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 px-2"
                            onClick={() => handleRemoveModelFromAll(model)}
                            disabled={addingModels}
                          >
                            移除
                          </Button>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
              <div className="space-y-2">
                <Textarea
                  value={customModelInput}
                  onChange={(e) => setCustomModelInput(e.target.value)}
                  placeholder="每行一个模型 ID，可用来自定义或补充上游未返回的模型"
                  className="h-28"
                />
                <div className="flex justify-end">
                  <Button onClick={handleAddCustomModels} disabled={addingModels || !allModelsProvider}>
                    {addingModels ? "提交中..." : "添加到全部模型"}
                  </Button>
                </div>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button onClick={() => setAllModelsOpen(false)}>关闭</Button>
          </DialogFooter>
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
                : `上游返回 ${providerModels.length} 个，已缓存 ${getAllModelsForProvider(modelsOpenId || 0).length} 个`}
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
                        className="flex items-center justify-between p-2 border rounded-lg"
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
                            <div className="font-medium truncate">{model.id}</div>
                            <div className="text-xs text-muted-foreground">
                              {isSaved ? "已在全部模型" : "未添加"}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {isSaved ? (
                            <span className="text-xs text-green-600">已缓存</span>
                          ) : null}
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
 
