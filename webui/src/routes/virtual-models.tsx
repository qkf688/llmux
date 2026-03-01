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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import Loading from "@/components/loading";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import {
  getVirtualModels,
  createVirtualModel,
  updateVirtualModel,
  deleteVirtualModel,
  getVirtualModelMappings,
  createVirtualModelMapping,
  updateVirtualModelMapping,
  deleteVirtualModelMapping,
  batchCreateVirtualModelMapping,
  getModels,
  getProviders,
  updateProvider,
  type VirtualModel,
  type VirtualModelMapping,
  type Model,
} from "@/lib/api";

// 表单验证模式
const formSchema = z.object({
  name: z.string().min(1, { message: "虚拟模型名称不能为空" }),
  description: z.string(),
  strategy: z.enum(["priority", "round_robin", "random"]),
  max_retry: z.number().min(0, { message: "重试次数不能为负数" }),
  time_out: z.number().min(0, { message: "超时时间不能为负数" }),
  io_log: z.boolean(),
  enabled: z.boolean(),
});

// 映射表单验证模式
const mappingFormSchema = z.object({
  real_model_id: z.number().min(1, { message: "请选择真实模型" }),
  priority: z.number().min(0, { message: "优先级不能为负数" }),
  weight: z.number().min(1, { message: "权重必须大于0" }),
  enabled: z.boolean(),
});

export default function VirtualModelsPage() {
  const [virtualModels, setVirtualModels] = useState<VirtualModel[]>([]);
  const [realModels, setRealModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<VirtualModel | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  // 映射管理状态
  const [mappingsDialogOpen, setMappingsDialogOpen] = useState(false);
  const [currentVirtualModel, setCurrentVirtualModel] = useState<VirtualModel | null>(null);
  const [mappings, setMappings] = useState<VirtualModelMapping[]>([]);
  const [mappingDialogOpen, setMappingDialogOpen] = useState(false);
  const [editingMapping, setEditingMapping] = useState<VirtualModelMapping | null>(null);

  // 拉黑管理状态
  const [blacklistDialogOpen, setBlacklistDialogOpen] = useState(false);
  const [blacklistedProviders, setBlacklistedProviders] = useState<any[]>([]);
  const [providers, setProviders] = useState<any[]>([]);
  const [selectProviderDialogOpen, setSelectProviderDialogOpen] = useState(false);
  const [selectedProviders, setSelectedProviders] = useState<number[]>([]);
  const [providerSearchQuery, setProviderSearchQuery] = useState("");

  // 批量添加状态
  const [batchDialogOpen, setBatchDialogOpen] = useState(false);
  const [selectedModelIds, setSelectedModelIds] = useState<number[]>([]);
  const [batchPriority, setBatchPriority] = useState(10);
  const [batchWeight, setBatchWeight] = useState(5);
  const [batchEnabled, setBatchEnabled] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");

  // 初始化表单
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      description: "",
      strategy: "priority",
      max_retry: 10,
      time_out: 60,
      io_log: false,
      enabled: true,
    },
  });

  // 映射表单
  const mappingForm = useForm<z.infer<typeof mappingFormSchema>>({
    resolver: zodResolver(mappingFormSchema),
    defaultValues: {
      real_model_id: 0,
      priority: 10,
      weight: 5,
      enabled: true,
    },
  });

  useEffect(() => {
    fetchData();
  }, []);

  const refreshProvidersState = async () => {
    const latestProviders = await getProviders();
    setProviders(latestProviders);
    setBlacklistedProviders(latestProviders.filter(p => p.blacklisted));
  };

  const fetchData = async () => {
    try {
      setLoading(true);
      const [vModels, rModels, pProviders] = await Promise.all([
        getVirtualModels(),
        getModels(),
        getProviders(),
      ]);
      setVirtualModels(vModels);
      setRealModels(rModels);
      setProviders(pProviders);
      setBlacklistedProviders(pProviders.filter(p => p.blacklisted));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取数据失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingModel(null);
    form.reset({
      name: "",
      description: "",
      strategy: "priority",
      max_retry: 10,
      time_out: 60,
      io_log: false,
      enabled: true,
    });
    setOpen(true);
  };

  const handleEdit = (model: VirtualModel) => {
    setEditingModel(model);
    form.reset({
      name: model.Name,
      description: model.Description,
      strategy: model.Strategy as "priority" | "round_robin" | "random",
      max_retry: model.MaxRetry,
      time_out: model.TimeOut,
      io_log: model.IOLog,
      enabled: model.Enabled,
    });
    setOpen(true);
  };

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      if (editingModel) {
        await updateVirtualModel(editingModel.ID, values);
        toast.success("虚拟模型更新成功");
      } else {
        await createVirtualModel(values);
        toast.success("虚拟模型创建成功");
      }
      setOpen(false);
      fetchData();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteVirtualModel(deleteId);
      toast.success("虚拟模型删除成功");
      setDeleteId(null);
      fetchData();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除失败: ${message}`);
    }
  };

  // 管理映射
  const handleManageMappings = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      const data = await getVirtualModelMappings(model.ID);
      setMappings(data);
      setMappingsDialogOpen(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取映射失败: ${message}`);
    }
  };

  const handleAddMapping = () => {
    setEditingMapping(null);
    mappingForm.reset({
      real_model_id: 0,
      priority: 10,
      weight: 5,
      enabled: true,
    });
    setMappingDialogOpen(true);
  };

  const handleEditMapping = (mapping: VirtualModelMapping) => {
    setEditingMapping(mapping);
    mappingForm.reset({
      real_model_id: mapping.RealModelID,
      priority: mapping.Priority,
      weight: mapping.Weight,
      enabled: mapping.Enabled,
    });
    setMappingDialogOpen(true);
  };

  const onMappingSubmit = async (values: z.infer<typeof mappingFormSchema>) => {
    if (!currentVirtualModel) return;
    try {
      if (editingMapping) {
        await updateVirtualModelMapping(currentVirtualModel.ID, editingMapping.ID, values);
        toast.success("映射更新成功");
      } else {
        await createVirtualModelMapping(currentVirtualModel.ID, values);
        toast.success("映射创建成功");
      }
      setMappingDialogOpen(false);
      const data = await getVirtualModelMappings(currentVirtualModel.ID);
      setMappings(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    }
  };

  const handleDeleteMapping = async (mappingId: number) => {
    if (!currentVirtualModel) return;
    try {
      await deleteVirtualModelMapping(currentVirtualModel.ID, mappingId);
      toast.success("映射删除成功");
      const data = await getVirtualModelMappings(currentVirtualModel.ID);
      setMappings(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除失败: ${message}`);
    }
  };

  // 批量添加映射
  const handleBatchAdd = () => {
    setSelectedModelIds([]);
    setBatchPriority(10);
    setBatchWeight(5);
    setBatchEnabled(true);
    setSearchQuery("");
    setBatchDialogOpen(true);
  };

  const handleBatchSubmit = async () => {
    if (!currentVirtualModel) return;
    if (selectedModelIds.length === 0) {
      toast.error("请至少选择一个真实模型");
      return;
    }

    try {
      const mappingsToCreate = selectedModelIds.map(modelId => ({
        real_model_id: modelId,
        priority: batchPriority,
        weight: batchWeight,
        enabled: batchEnabled,
      }));

      const result = await batchCreateVirtualModelMapping(currentVirtualModel.ID, mappingsToCreate);

      if (result.success_count > 0) {
        toast.success(`成功添加 ${result.success_count} 个映射`);
      }
      if (result.failed_count > 0) {
        toast.error(`${result.failed_count} 个映射添加失败`);
        result.failed_items.forEach(item => {
          const modelName = getRealModelName(item.real_model_id);
          toast.error(`${modelName}: ${item.reason}`);
        });
      }

      setBatchDialogOpen(false);
      const data = await getVirtualModelMappings(currentVirtualModel.ID);
      setMappings(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`批量添加失败: ${message}`);
    }
  };

  const toggleModelSelection = (modelId: number) => {
    setSelectedModelIds(prev =>
      prev.includes(modelId)
        ? prev.filter(id => id !== modelId)
        : [...prev, modelId]
    );
  };

  const handleSelectAll = () => {
    const filteredModels = getFilteredModels();
    const unmappedModels = filteredModels.filter(
      model => !mappedModelIds.has(model.ID)
    );
    setSelectedModelIds(unmappedModels.map(m => m.ID));
  };

  const handleInvertSelection = () => {
    const filteredModels = getFilteredModels();
    const unmappedModels = filteredModels.filter(
      model => !mappedModelIds.has(model.ID)
    );
    setSelectedModelIds(prev => {
      const currentSet = new Set(prev);
      return unmappedModels
        .filter(m => !currentSet.has(m.ID))
        .map(m => m.ID);
    });
  };

  const handleClearSelection = () => {
    setSelectedModelIds([]);
  };

  const getFilteredModels = () => {
    if (!searchQuery) return realModels;
    return realModels.filter(model =>
      model.Name.toLowerCase().includes(searchQuery.toLowerCase())
    );
  };

  const mappedModelIds = new Set(mappings.map(m => m.RealModelID));

  const getStrategyLabel = (strategy: string) => {
    const labels: Record<string, string> = {
      priority: "优先级+权重",
      round_robin: "轮询",
      random: "随机",
    };
    return labels[strategy] || strategy;
  };

  const getRealModelName = (modelId: number) => {
    const model = realModels.find(m => m.ID === modelId);
    return model ? model.Name : `ID: ${modelId}`;
  };

  // 拉黑管理
  const handleManageBlacklist = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      await refreshProvidersState();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商数据失败: ${message}`);
      const blacklisted = providers.filter(p => p.blacklisted);
      setBlacklistedProviders(blacklisted);
    }
    setBlacklistDialogOpen(true);
  };

  const handleAddBlacklistedProvider = () => {
    setSelectedProviders([]);
    setProviderSearchQuery("");
    setSelectProviderDialogOpen(true);
  };

  const handleToggleProviderSelection = (providerId: number) => {
    setSelectedProviders(prev =>
      prev.includes(providerId)
        ? prev.filter(id => id !== providerId)
        : [...prev, providerId]
    );
  };

  const handleConfirmAddBlacklistedProviders = async () => {
    if (selectedProviders.length === 0) {
      toast.error("请至少选择一个提供商");
      return;
    }

    try {
      for (const providerId of selectedProviders) {
        await updateProvider(providerId, { blacklisted: true });
      }
      toast.success(`成功拉黑 ${selectedProviders.length} 个提供商`);
      setSelectProviderDialogOpen(false);
      await refreshProvidersState();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    }
  };

  const handleRemoveBlacklistedProvider = async (providerId: number) => {
    try {
      await updateProvider(providerId, { blacklisted: false });
      toast.success("提供商已解除拉黑");
      await refreshProvidersState();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    }
  };

  const getFilteredProviders = () => {
    if (!providerSearchQuery) return providers;
    return providers.filter(provider =>
      provider.Name.toLowerCase().includes(providerSearchQuery.toLowerCase())
    );
  };

  if (loading) {
    return <Loading />;
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold">虚拟模型管理</h1>
          <p className="text-sm text-muted-foreground mt-1">
            将多个真实模型组合成虚拟模型，支持多种路由策略
          </p>
        </div>
        <Button onClick={handleCreate}>创建虚拟模型</Button>
      </div>

      <div className="border rounded-lg">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>描述</TableHead>
              <TableHead>路由策略</TableHead>
              <TableHead>重试/超时</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {virtualModels.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  暂无虚拟模型
                </TableCell>
              </TableRow>
            ) : (
              virtualModels.map((model) => (
                <TableRow key={model.ID}>
                  <TableCell className="font-medium">{model.Name}</TableCell>
                  <TableCell className="max-w-xs truncate">{model.Description || "-"}</TableCell>
                  <TableCell>{getStrategyLabel(model.Strategy)}</TableCell>
                  <TableCell>{model.MaxRetry} / {model.TimeOut}s</TableCell>
                  <TableCell>
                    <span className={`px-2 py-1 rounded text-xs ${model.Enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"}`}>
                      {model.Enabled ? "启用" : "禁用"}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-2">
                      <Button variant="outline" size="sm" onClick={() => handleManageMappings(model)}>
                        管理映射
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => handleManageBlacklist(model)}>
                        拉黑管理
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => handleEdit(model)}>
                        编辑
                      </Button>
                      <Button variant="destructive" size="sm" onClick={() => setDeleteId(model.ID)}>
                        删除
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* 创建/编辑对话框 */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{editingModel ? "编辑虚拟模型" : "创建虚拟模型"}</DialogTitle>
            <DialogDescription>
              配置虚拟模型的基本信息和路由策略
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input placeholder="例如: gpt-4-balanced" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>描述</FormLabel>
                    <FormControl>
                      <Textarea placeholder="虚拟模型的用途说明" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="strategy"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>路由策略</FormLabel>
                    <Select onValueChange={field.onChange} defaultValue={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="选择路由策略" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="priority">优先级+权重</SelectItem>
                        <SelectItem value="round_robin">轮询</SelectItem>
                        <SelectItem value="random">随机</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="max_retry"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>最大重试次数</FormLabel>
                      <FormControl>
                        <Input type="number" {...field} onChange={e => field.onChange(parseInt(e.target.value))} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="time_out"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>超时时间（秒）</FormLabel>
                      <FormControl>
                        <Input type="number" {...field} onChange={e => field.onChange(parseInt(e.target.value))} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <div className="flex gap-4">
                <FormField
                  control={form.control}
                  name="io_log"
                  render={({ field }) => (
                    <FormItem className="flex items-center gap-2">
                      <FormLabel>记录 IO 日志</FormLabel>
                      <FormControl>
                        <Switch checked={field.value} onCheckedChange={field.onChange} />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="enabled"
                  render={({ field }) => (
                    <FormItem className="flex items-center gap-2">
                      <FormLabel>启用</FormLabel>
                      <FormControl>
                        <Switch checked={field.value} onCheckedChange={field.onChange} />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                  取消
                </Button>
                <Button type="submit">
                  {editingModel ? "更新" : "创建"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 删除确认对话框 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              此操作将删除虚拟模型及其所有映射关系，且无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 映射管理对话框 */}
      <Dialog open={mappingsDialogOpen} onOpenChange={setMappingsDialogOpen}>
        <DialogContent className="max-w-3xl w-[88vw] h-[82vh] max-h-[92vh] flex flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>管理映射 - {currentVirtualModel?.Name}</DialogTitle>
            <DialogDescription>
              配置虚拟模型关联的真实模型
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2 flex-1 min-h-0">
            <div className="flex gap-2">
              <Button size="sm" className="h-8 px-3" onClick={handleAddMapping}>添加映射</Button>
              <Button size="sm" className="h-8 px-3" variant="outline" onClick={handleBatchAdd}>批量添加</Button>
            </div>
            <div className="border rounded-md flex-1 min-h-0 overflow-y-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="py-2 text-xs">真实模型</TableHead>
                    <TableHead className="py-2 text-xs">优先级</TableHead>
                    <TableHead className="py-2 text-xs">权重</TableHead>
                    <TableHead className="py-2 text-xs">状态</TableHead>
                    <TableHead className="py-2 text-xs">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {mappings.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="py-3 text-center text-muted-foreground text-sm">
                        暂无映射
                      </TableCell>
                    </TableRow>
                  ) : (
                    mappings.map((mapping) => (
                      <TableRow key={mapping.ID}>
                        <TableCell className="py-2 text-sm">{getRealModelName(mapping.RealModelID)}</TableCell>
                        <TableCell className="py-2 text-sm">{mapping.Priority}</TableCell>
                        <TableCell className="py-2 text-sm">{mapping.Weight}</TableCell>
                        <TableCell className="py-2">
                          <span className={`px-2 py-0.5 rounded text-xs ${mapping.Enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"}`}>
                            {mapping.Enabled ? "启用" : "禁用"}
                          </span>
                        </TableCell>
                        <TableCell className="py-2">
                          <div className="flex gap-1.5">
                            <Button variant="outline" size="sm" className="h-7 px-2" onClick={() => handleEditMapping(mapping)}>
                              编辑
                            </Button>
                            <Button variant="destructive" size="sm" className="h-7 px-2" onClick={() => handleDeleteMapping(mapping.ID)}>
                              删除
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* 映射创建/编辑对话框 */}
      <Dialog open={mappingDialogOpen} onOpenChange={setMappingDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingMapping ? "编辑映射" : "添加映射"}</DialogTitle>
          </DialogHeader>
          <Form {...mappingForm}>
            <form onSubmit={mappingForm.handleSubmit(onMappingSubmit)} className="space-y-4">
              <FormField
                control={mappingForm.control}
                name="real_model_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>真实模型</FormLabel>
                    <Select onValueChange={(value) => field.onChange(parseInt(value))} defaultValue={field.value.toString()}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="选择真实模型" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {realModels.map((model) => (
                          <SelectItem key={model.ID} value={model.ID.toString()}>
                            {model.Name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  control={mappingForm.control}
                  name="priority"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>优先级</FormLabel>
                      <FormControl>
                        <Input type="number" {...field} onChange={e => field.onChange(parseInt(e.target.value))} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={mappingForm.control}
                  name="weight"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>权重</FormLabel>
                      <FormControl>
                        <Input type="number" {...field} onChange={e => field.onChange(parseInt(e.target.value))} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={mappingForm.control}
                name="enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center gap-2">
                    <FormLabel>启用</FormLabel>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setMappingDialogOpen(false)}>
                  取消
                </Button>
                <Button type="submit">
                  {editingMapping ? "更新" : "添加"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 批量添加映射对话框 */}
      <Dialog open={batchDialogOpen} onOpenChange={setBatchDialogOpen}>
        <DialogContent className="max-w-3xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>批量添加映射</DialogTitle>
            <DialogDescription>
              选择多个真实模型并设置统一参数
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {/* 搜索框 */}
            <div>
              <Input
                placeholder="搜索模型名称..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>

            {/* 快捷操作按钮 */}
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleSelectAll}>
                全选
              </Button>
              <Button variant="outline" size="sm" onClick={handleInvertSelection}>
                反选
              </Button>
              <Button variant="outline" size="sm" onClick={handleClearSelection}>
                清空
              </Button>
              <span className="text-sm text-muted-foreground ml-auto self-center">
                已选择 {selectedModelIds.length} 个模型
              </span>
            </div>

            {/* 模型列表 */}
            <div className="border rounded-lg max-h-60 overflow-y-auto">
              <div className="divide-y">
                {getFilteredModels().map((model) => {
                  const isMapped = mappedModelIds.has(model.ID);
                  const isSelected = selectedModelIds.includes(model.ID);
                  return (
                    <div
                      key={model.ID}
                      className={`flex items-center gap-3 p-3 hover:bg-gray-50 ${
                        isMapped ? "bg-gray-100" : ""
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleModelSelection(model.ID)}
                        disabled={isMapped}
                        className="w-4 h-4"
                      />
                      <div className="flex-1">
                        <div className={`font-medium ${isMapped ? "text-gray-500 line-through" : ""}`}>
                          {model.Name}
                        </div>
                        {model.Remark && (
                          <div className="text-sm text-muted-foreground">{model.Remark}</div>
                        )}
                      </div>
                      {isMapped && (
                        <span className="text-xs text-gray-500 bg-gray-200 px-2 py-1 rounded">
                          已映射
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>

            {/* 统一参数设置 */}
            <div className="space-y-3 border-t pt-4">
              <h4 className="font-medium">统一参数</h4>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">优先级</label>
                  <Input
                    type="number"
                    value={batchPriority}
                    onChange={(e) => setBatchPriority(parseInt(e.target.value))}
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">权重</label>
                  <Input
                    type="number"
                    value={batchWeight}
                    onChange={(e) => setBatchWeight(parseInt(e.target.value))}
                  />
                </div>
              </div>
              <div className="flex items-center gap-2">
                <label className="text-sm font-medium">启用</label>
                <Switch checked={batchEnabled} onCheckedChange={setBatchEnabled} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setBatchDialogOpen(false)}>
              取消
            </Button>
            <Button onClick={handleBatchSubmit}>
              添加 ({selectedModelIds.length})
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 拉黑管理对话框 */}
      <Dialog open={blacklistDialogOpen} onOpenChange={setBlacklistDialogOpen}>
        <DialogContent className="max-w-3xl h-[560px] max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>拉黑管理</DialogTitle>
            <DialogDescription>
              管理已拉黑的提供商，拉黑后虚拟模型不会请求该提供商的模型
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 flex-1 min-h-0">
            <div className="flex justify-between items-center">
              <h4 className="font-medium">已拉黑提供商</h4>
              <Button onClick={handleAddBlacklistedProvider}>添加</Button>
            </div>
            <div className="flex-1 min-h-0 overflow-y-auto border rounded-md">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>名称</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {blacklistedProviders.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={3} className="text-center text-muted-foreground">
                        暂无拉黑提供商
                      </TableCell>
                    </TableRow>
                  ) : (
                    blacklistedProviders.map((provider) => (
                      <TableRow key={provider.ID}>
                        <TableCell>{provider.Name}</TableCell>
                        <TableCell>{provider.Type}</TableCell>
                        <TableCell>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleRemoveBlacklistedProvider(provider.ID)}
                          >
                            解除拉黑
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* 选择提供商对话框 */}
      <Dialog open={selectProviderDialogOpen} onOpenChange={setSelectProviderDialogOpen}>
        <DialogContent className="max-w-2xl w-[90vw] h-[78vh] max-h-[90vh] flex flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>选择提供商</DialogTitle>
            <DialogDescription>
              选择要拉黑的提供商
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2 flex-1 min-h-0">
            {/* 搜索框 */}
            <div>
              <Input
                className="h-9"
                placeholder="搜索提供商名称..."
                value={providerSearchQuery}
                onChange={(e) => setProviderSearchQuery(e.target.value)}
              />
            </div>

            {/* 提供商列表 */}
            <div className="border rounded-md flex-1 min-h-0 overflow-y-auto">
              <div className="divide-y">
                {getFilteredProviders().map((provider) => {
                  const isSelected = selectedProviders.includes(provider.ID);
                  const isBlacklisted = provider.blacklisted;
                  return (
                    <div
                      key={provider.ID}
                      className={`flex items-center gap-2 p-2 hover:bg-gray-50 ${
                        isBlacklisted ? "bg-gray-100" : ""
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => handleToggleProviderSelection(provider.ID)}
                        disabled={isBlacklisted}
                        className="w-3.5 h-3.5"
                      />
                      <div className="flex-1">
                        <div className={`text-sm font-medium ${isBlacklisted ? "text-gray-500 line-through" : ""}`}>
                          {provider.Name}
                        </div>
                        <div className="text-xs text-muted-foreground">{provider.Type}</div>
                      </div>
                      {isBlacklisted && (
                        <span className="text-xs text-gray-500 bg-gray-200 px-1.5 py-0.5 rounded">
                          已拉黑
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setSelectProviderDialogOpen(false)}>
              取消
            </Button>
            <Button onClick={handleConfirmAddBlacklistedProviders}>
              添加 ({selectedProviders.length})
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
