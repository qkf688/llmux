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

function extractCustomModels(config: string): string[] {
  try {
    const parsed = JSON.parse(config);
    if (!Array.isArray(parsed.custom_models)) {
      return [];
    }
    return parsed.custom_models
      .filter((item: unknown) => typeof item === "string")
      .map((item: string) => item.trim())
      .filter(Boolean);
  } catch {
    return [];
  }
}

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
  const [customModelsOpen, setCustomModelsOpen] = useState(false);
  const [customModelsList, setCustomModelsList] = useState<string[]>([]);
  const [customModelsProviderName, setCustomModelsProviderName] = useState<string>("");

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

  const fetchProviderModels = async (providerId: number) => {
    try {
      setModelsLoading(true);
      const data = await getProviderModels(providerId);
      // 确保 data 是数组，防止后端返回 null 导致白屏
      const models = Array.isArray(data) ? data : [];
      setProviderModels(models);
      setFilteredProviderModels(models);
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
    await fetchProviderModels(providerId);
  };

  const copyModelName = async (modelName: string) => {
    await navigator.clipboard.writeText(modelName);
    toast.success(`已复制模型名称: ${modelName}`);
  };

  const openCustomModelsDialog = (provider: Provider) => {
    const customModels = extractCustomModels(provider.Config);
    setCustomModelsList(customModels);
    setCustomModelsProviderName(provider.Name);
    setCustomModelsOpen(true);
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    try {
      const config = buildConfigFromForm(values);
      await createProvider({
        name: values.name,
        type: values.type,
        config: config,
        console: values.console || ""
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 创建成功`);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "" });
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
        console: values.console || ""
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 更新成功`);
      setEditingProvider(null);
      form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "" });
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
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    form.reset({ name: "", type: "", base_url: "", api_key: "", beta: "", version: "", console: "", custom_models: "" });
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const hasFilter = nameFilter.trim() !== "" || typeFilter !== "all";

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
                    <TableHead>自定义模型</TableHead>
                    <TableHead>控制台</TableHead>
                    <TableHead className="w-[260px]">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {providers.map((provider) => {
                    const customModels = extractCustomModels(provider.Config);
                    return (
                      <TableRow key={provider.ID}>
                        <TableCell className="font-mono text-xs text-muted-foreground">{provider.ID}</TableCell>
                        <TableCell className="font-medium">{provider.Name}</TableCell>
                        <TableCell className="text-sm">{provider.Type}</TableCell>
                        <TableCell>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => openCustomModelsDialog(provider)}
                            disabled={customModels.length === 0}
                          >
                            查看{customModels.length > 0 ? `（${customModels.length}）` : ""}
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
                const customModels = extractCustomModels(provider.Config);
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
                          className="h-7 px-2 text-xs mt-1"
                          onClick={() => openCustomModelsDialog(provider)}
                          disabled={customModels.length === 0}
                        >
                          自定义模型{customModels.length > 0 ? `（${customModels.length}）` : ""}
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
                      <Input {...field} type="password" placeholder="sk-..." />
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
                    <FormLabel>自定义模型（可选）</FormLabel>
                    <FormControl>
                      <Textarea {...field} placeholder="每行一个模型 ID，优先使用此列表，无需从提供商获取" className="h-28" />
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

      {/* 自定义模型对话框 */}
      <Dialog open={customModelsOpen} onOpenChange={setCustomModelsOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{customModelsProviderName || "当前提供商"}的自定义模型</DialogTitle>
            <DialogDescription>展示该提供商配置的自定义模型列表</DialogDescription>
          </DialogHeader>

          <div className="max-h-80 overflow-y-auto">
            {customModelsList.length === 0 ? (
              <div className="text-center text-muted-foreground py-6">未配置自定义模型</div>
            ) : (
              <div className="space-y-2">
                {customModelsList.map((model, index) => (
                  <div
                    key={`${model}-${index}`}
                    className="flex items-center justify-between rounded-md border px-3 py-2"
                  >
                    <span className="font-mono text-sm text-primary">{model}</span>
                    <Button variant="ghost" size="sm" onClick={() => copyModelName(model)}>
                      复制
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <DialogFooter>
            <Button onClick={() => setCustomModelsOpen(false)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 模型列表对话框 */}
      <Dialog open={modelsOpen} onOpenChange={setModelsOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{providers.find(v => v.ID === modelsOpenId)?.Name}模型列表</DialogTitle>
            <DialogDescription>
              当前提供商的所有可用模型
            </DialogDescription>
          </DialogHeader>

          {/* 搜索框 */}
          {!modelsLoading && providerModels.length > 0 && (
            <div className="mb-4">
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
            <div className="max-h-96 overflow-y-auto">
              {filteredProviderModels.length === 0 ? (
                <div className="text-center text-gray-500 py-8">
                  {providerModels.length === 0 ? '暂无模型数据' : '未找到匹配的模型'}
                </div>
              ) : (
                <div className="space-y-2">
                  {filteredProviderModels.map((model, index) => (
                    <div
                      key={index}
                      className="flex items-center justify-between p-2 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="font-medium">{model.id}</div>
                      </div>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => copyModelName(model.id)}
                              className="min-w-12"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" aria-hidden="true" className="h-4 w-4"><path stroke-linecap="round" stroke-linejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path></svg>
                            </Button>
                          </TooltipTrigger>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  ))}
                </div>
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
 
