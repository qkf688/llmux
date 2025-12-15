import { useState, useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
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
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import Loading from "@/components/loading";
import { Spinner } from "@/components/ui/spinner";
import { ChevronDown, ChevronRight } from "lucide-react";
import {
  getModels,
  createModel,
  updateModel,
  deleteModel,
  batchDeleteModels,
  getProviders,
} from "@/lib/api";
import type { Model, Provider, ProviderModel } from "@/lib/api";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";

type MobileInfoItemProps = {
  label: string;
  value: ReactNode;
};

type ProviderModelWithOwner = ProviderModel & {
  providerId: number;
  providerName: string;
};

type ProviderModelGroup = {
  provider: Provider;
  models: ProviderModelWithOwner[];
};

const MobileInfoItem = ({ label, value }: MobileInfoItemProps) => (
  <div className="space-y-1">
    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
    <div className="text-sm font-medium break-words">{value}</div>
  </div>
);

// 定义表单验证模式
const formSchema = z.object({
  name: z.string().min(1, { message: "模型名称不能为空" }),
  remark: z.string(),
  max_retry: z.number().min(0, { message: "重试次数限制不能为负数" }),
  time_out: z.number().min(0, { message: "超时时间不能为负数" }),
  io_log: z.boolean(),
});

export default function ModelsPage() {
  const navigate = useNavigate();
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<Model | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [togglingIOLog, setTogglingIOLog] = useState<Record<number, boolean>>({});

  // 供应商相关状态
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProviderId, setSelectedProviderId] = useState<string>("all");
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [loadingProviderModels, setLoadingProviderModels] = useState(false);
  const [collapsedProviders, setCollapsedProviders] = useState<Record<number, boolean>>({});
  
  // 模型选择弹窗状态
  const [modelPickerOpen, setModelPickerOpen] = useState(false);
  const [modelSearchQuery, setModelSearchQuery] = useState("");

  // 初始化表单
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      remark: "",
      max_retry: 10,
      time_out: 60,
      io_log: false,
    },
  });

  useEffect(() => {
    fetchModels();
    fetchProviders();
  }, []);

  const fetchModels = async () => {
    try {
      setLoading(true);
      const data = await getModels();
      setModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型列表失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchProviders = async () => {
    try {
      setLoadingProviderModels(true);
      const data = await getProviders();
      setProviders(data);
      const groups = data.map((provider) => {
        const models = toProviderModelList(parseAllModelsFromConfig(provider.Config)).map((model) => ({
          ...model,
          providerId: provider.ID,
          providerName: provider.Name,
        }));
        return { provider, models };
      });
      setProviderModelGroups(groups);
      setProviderModels(groups.flatMap((group) => group.models));
      setCollapsedProviders((prev) => {
        const next: Record<number, boolean> = {};
        groups.forEach(({ provider }) => {
          next[provider.ID] = prev[provider.ID] ?? false;
        });
        return next;
      });
    } catch (err) {
      console.error("获取供应商列表失败:", err);
    } finally {
      setLoadingProviderModels(false);
    }
  };

  const handleSelectProviderModel = (modelId: string) => {
    form.setValue("name", modelId);
    setModelPickerOpen(false);
    setModelSearchQuery("");
  };

  // 过滤模型列表
  const toggleProviderCollapse = (providerId: number) => {
    setCollapsedProviders((prev) => ({
      ...prev,
      [providerId]: !prev[providerId],
    }));
  };

  const filteredProviderGroups = providerModelGroups
    .filter((group) => selectedProviderId === "all" || group.provider.ID.toString() === selectedProviderId)
    .map((group) => ({
      ...group,
      models: group.models.filter((model) =>
        model.id.toLowerCase().includes(modelSearchQuery.toLowerCase())
      )
    }))
    .filter((group) => group.models.length > 0);
  const filteredProviderModels = filteredProviderGroups.flatMap((group) => group.models);

  // 打开模型选择弹窗
  const openModelPicker = () => {
    if (providerModels.length === 0) {
      toast.error("暂无任何“全部模型”，请先在提供商管理页同步或添加模型");
      return;
    }
    setModelPickerOpen(true);
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    try {
      await createModel(values);
      setOpen(false);
      toast.success(`模型: ${values.name} 创建成功`);
      form.reset({ name: "", remark: "", max_retry: 10, time_out: 60, io_log: false });
      fetchModels();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建模型失败: ${message}`);
    }
  };

  const handleUpdate = async (values: z.infer<typeof formSchema>) => {
    if (!editingModel) return;
    try {
      await updateModel(editingModel.ID, values);
      setOpen(false);
      toast.success(`模型: ${values.name} 更新成功`);
      setEditingModel(null);
      form.reset({ name: "", remark: "", max_retry: 10, time_out: 60, io_log: false });
      fetchModels();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新模型失败: ${message}`);
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      const targetModel = models.find((model) => model.ID === deleteId);
      await deleteModel(deleteId);
      setDeleteId(null);
      fetchModels();
      toast.success(`模型: ${targetModel?.Name ?? deleteId} 删除成功`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除模型失败: ${message}`);
      console.error(err);
    }
  };

  const openEditDialog = (model: Model) => {
    setEditingModel(model);
    form.reset({
      name: model.Name,
      remark: model.Remark,
      max_retry: model.MaxRetry,
      time_out: model.TimeOut,
      io_log: model.IOLog,
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingModel(null);
    form.reset({ name: "", remark: "", max_retry: 10, time_out: 60, io_log: false });
    setSelectedProviderId("all");
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(models.map(model => model.ID));
    } else {
      setSelectedIds([]);
    }
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    if (checked) {
      setSelectedIds([...selectedIds, id]);
    } else {
      setSelectedIds(selectedIds.filter(selectedId => selectedId !== id));
    }
  };

  const handleBatchDelete = async () => {
    if (selectedIds.length === 0) return;
    setBatchDeleting(true);
    try {
      const result = await batchDeleteModels(selectedIds);
      toast.success(`成功删除 ${result.deleted} 个模型`);
      setSelectedIds([]);
      setBatchDeleteDialogOpen(false);
      fetchModels();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`批量删除模型失败: ${message}`);
    } finally {
      setBatchDeleting(false);
    }
  };

  const handleToggleIOLog = async (model: Model) => {
    const modelId = model.ID;
    const newIOLogValue = !model.IOLog;

    // 设置加载状态
    setTogglingIOLog(prev => ({ ...prev, [modelId]: true }));

    try {
      await updateModel(modelId, {
        name: model.Name,
        remark: model.Remark,
        max_retry: model.MaxRetry,
        time_out: model.TimeOut,
        io_log: newIOLogValue,
      });

      // 更新本地状态
      setModels(prevModels =>
        prevModels.map(m =>
          m.ID === modelId ? { ...m, IOLog: newIOLogValue } : m
        )
      );

      toast.success(`模型 ${model.Name} 的 IO 记录已${newIOLogValue ? '开启' : '关闭'}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`切换 IO 记录失败: ${message}`);
    } finally {
      // 清除加载状态
      setTogglingIOLog(prev => {
        const next = { ...prev };
        delete next[modelId];
        return next;
      });
    }
  };

  const isAllSelected = models.length > 0 && selectedIds.length === models.length;
  const isPartialSelected = selectedIds.length > 0 && selectedIds.length < models.length;

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">模型管理</h2>
          </div>
          <div className="flex w-full sm:w-auto items-center justify-end gap-2">
            {selectedIds.length > 0 && (
              <AlertDialog open={batchDeleteDialogOpen} onOpenChange={setBatchDeleteDialogOpen}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" className="w-full sm:w-auto">
                    批量删除 ({selectedIds.length})
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>确定要批量删除这些模型吗？</AlertDialogTitle>
                    <AlertDialogDescription>
                      此操作无法撤销。这将永久删除选中的 {selectedIds.length} 个模型。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={batchDeleting}>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={handleBatchDelete} disabled={batchDeleting}>
                      {batchDeleting ? "删除中..." : "确认删除"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
            <Button onClick={openCreateDialog} className="w-full sm:w-auto sm:min-w-[120px]">
              添加模型
            </Button>
          </div>
        </div>
      </div>
      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载模型列表" />
          </div>
        ) : models.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            暂无模型数据
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="hidden sm:block w-full overflow-x-auto">
              <Table className="min-w-[900px]">
                <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
                  <TableRow>
                    <TableHead className="w-[50px]">
                      <Checkbox
                        checked={isAllSelected}
                        ref={(el) => {
                          if (el) {
                            (el as unknown as HTMLInputElement).indeterminate = isPartialSelected;
                          }
                        }}
                        onCheckedChange={handleSelectAll}
                        aria-label="全选"
                      />
                    </TableHead>
                    <TableHead>ID</TableHead>
                    <TableHead>名称</TableHead>
                    <TableHead>备注</TableHead>
                    <TableHead>重试次数限制</TableHead>
                    <TableHead>超时时间(秒)</TableHead>
                    <TableHead>IO 记录</TableHead>
                    <TableHead className="w-[220px]">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {models.map((model) => (
                    <TableRow key={model.ID}>
                      <TableCell>
                        <Checkbox
                          checked={selectedIds.includes(model.ID)}
                          onCheckedChange={(checked) => handleSelectOne(model.ID, !!checked)}
                          aria-label={`选择 ${model.Name}`}
                        />
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">{model.ID}</TableCell>
                      <TableCell className="font-medium">{model.Name}</TableCell>
                      <TableCell className="max-w-[240px] truncate text-sm" title={model.Remark}>
                        {model.Remark || "-"}
                      </TableCell>
                      <TableCell>{model.MaxRetry}</TableCell>
                      <TableCell>{model.TimeOut}</TableCell>
                      <TableCell>
                        <Switch
                          checked={model.IOLog}
                          onCheckedChange={() => handleToggleIOLog(model)}
                          disabled={togglingIOLog[model.ID]}
                        />
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => navigate(`/model-providers?modelId=${model.ID}`)}
                          >
                            关联
                          </Button>
                          <Button variant="outline" size="sm" onClick={() => openEditDialog(model)}>
                            编辑
                          </Button>
                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button variant="destructive" size="sm" onClick={() => openDeleteDialog(model.ID)}>
                                删除
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>确定要删除这个模型吗？</AlertDialogTitle>
                                <AlertDialogDescription>此操作无法撤销。这将永久删除该模型。</AlertDialogDescription>
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
                  ))}
                </TableBody>
              </Table>
            </div>
            <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-3 divide-y divide-border">
              {/* 移动端全选 */}
              <div className="py-2 flex items-center gap-2 border-b">
                <Checkbox
                  checked={isAllSelected}
                  ref={(el) => {
                    if (el) {
                      (el as unknown as HTMLInputElement).indeterminate = isPartialSelected;
                    }
                  }}
                  onCheckedChange={handleSelectAll}
                  aria-label="全选"
                />
                <span className="text-sm text-muted-foreground">
                  {selectedIds.length > 0 ? `已选择 ${selectedIds.length} 项` : "全选"}
                </span>
              </div>
              {models.map((model) => (
                <div key={model.ID} className="py-3 space-y-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <Checkbox
                        checked={selectedIds.includes(model.ID)}
                        onCheckedChange={(checked) => handleSelectOne(model.ID, !!checked)}
                        aria-label={`选择 ${model.Name}`}
                      />
                      <div className="min-w-0 flex-1">
                        <h3 className="font-semibold text-sm truncate">{model.Name}</h3>
                        <p className="text-[11px] text-muted-foreground">ID: {model.ID}</p>
                      </div>
                    </div>
                  </div>
                  <div className="flex flex-wrap justify-end gap-1.5">
                    <Button variant="secondary" size="sm" className="h-7 px-2 text-xs" onClick={() => navigate(`/model-providers?modelId=${model.ID}`)}>
                      关联
                    </Button>
                    <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => openEditDialog(model)}>
                      编辑
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm" className="h-7 px-2 text-xs" onClick={() => openDeleteDialog(model.ID)}>
                          删除
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>确定要删除这个模型吗？</AlertDialogTitle>
                          <AlertDialogDescription>此操作无法撤销。这将永久删除该模型。</AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                          <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                  <div className="text-xs space-y-1">
                    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">备注</p>
                    <p className="break-words">{model.Remark || "-"}</p>
                  </div>
                  <div className="grid grid-cols-2 gap-3 text-xs">
                    <MobileInfoItem label="重试次数" value={model.MaxRetry} />
                    <MobileInfoItem label="超时时间" value={`${model.TimeOut} 秒`} />
                    <div className="space-y-1">
                      <p className="text-[11px] text-muted-foreground uppercase tracking-wide">IO 记录</p>
                      <div className="flex items-center">
                        <Switch
                          checked={model.IOLog}
                          onCheckedChange={() => handleToggleIOLog(model)}
                          disabled={togglingIOLog[model.ID]}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingModel ? "编辑模型" : "添加模型"}
            </DialogTitle>
            <DialogDescription>
              {editingModel
                ? "修改模型信息"
                : "添加一个新的模型"}
            </DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(editingModel ? handleUpdate : handleCreate)} className="space-y-4">
              {/* 供应商选择（仅在创建模式下显示） */}
              {!editingModel && (
                <div className="space-y-2">
                  <label className="text-sm font-medium">从提供商选择（默认全部）</label>
                  <div className="flex gap-2">
                    <Select
                      value={selectedProviderId}
                      onValueChange={setSelectedProviderId}
                    >
                      <SelectTrigger className="flex-1">
                        <SelectValue placeholder="选择供应商" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">全部</SelectItem>
                        {providers.map((provider) => (
                          <SelectItem key={provider.ID} value={provider.ID.toString()}>
                            {provider.Name} ({provider.Type})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={openModelPicker}
                      disabled={loadingProviderModels || providerModels.length === 0}
                    >
                      {loadingProviderModels ? (
                        <>
                          <Spinner className="h-4 w-4 mr-2" />
                          加载中
                        </>
                      ) : (
                        "选择模型"
                      )}
                    </Button>
                  </div>
                </div>
              )}

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
                name="remark"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>备注</FormLabel>
                    <FormControl>
                      <Textarea {...field} rows={3} />
                    </FormControl>
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
                      <FormLabel>重试次数限制</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          {...field}
                          onChange={e => field.onChange(+e.target.value)}
                        />
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
                      <FormLabel>超时时间(秒)</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          {...field}
                          onChange={e => field.onChange(+e.target.value)}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="io_log"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                    <div className="space-y-0.5">
                      <FormLabel className="text-base">IO 记录</FormLabel>
                      <div className="text-sm text-muted-foreground">
                        是否记录输入输出日志
                      </div>
                    </div>
                    <FormControl>
                      <Checkbox
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
                  {editingModel ? "更新" : "创建"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 模型选择弹窗 */}
      <Dialog open={modelPickerOpen} onOpenChange={setModelPickerOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>选择模型</DialogTitle>
            <DialogDescription>
              {selectedProviderId === "all"
                ? "从全部提供商的模型缓存中选择"
                : `从供应商 ${providers.find(p => p.ID.toString() === selectedProviderId)?.Name ?? "未找到"} 的模型缓存中选择`}
            </DialogDescription>
          </DialogHeader>
          
          {/* 搜索框 */}
          <div className="py-2">
            <Input
              placeholder="搜索模型名称..."
              value={modelSearchQuery}
              onChange={(e) => setModelSearchQuery(e.target.value)}
              className="w-full"
            />
          </div>
          
          {/* 模型列表 */}
          <div className="flex-1 min-h-0 overflow-y-auto border rounded-md space-y-2 p-2">
            {loadingProviderModels ? (
              <div className="flex items-center justify-center h-32">
                <Spinner className="h-6 w-6 mr-2" />
                <span>加载模型列表...</span>
              </div>
            ) : providerModels.length === 0 ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                暂无全部模型缓存，请先在提供商管理页同步
              </div>
            ) : filteredProviderGroups.length === 0 ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                没有找到匹配的模型
              </div>
            ) : (
              filteredProviderGroups.map((group) => {
                const isCollapsed = collapsedProviders[group.provider.ID] ?? false;
                return (
                  <div key={group.provider.ID} className="border rounded-md">
                    <button
                      type="button"
                      className="flex w-full items-center justify-between px-3 py-2 text-left hover:bg-muted/70 transition"
                      onClick={() => toggleProviderCollapse(group.provider.ID)}
                    >
                      <div className="flex items-center gap-2">
                        {isCollapsed ? (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <ChevronDown className="h-4 w-4 text-muted-foreground" />
                        )}
                        <span className="font-medium">{group.provider.Name}</span>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {group.models.length} 个模型
                      </span>
                    </button>
                    {!isCollapsed && (
                      <div className="divide-y">
                        {group.models.map((model) => (
                          <div
                            key={`${group.provider.ID}-${model.id}`}
                            className="p-3 hover:bg-muted cursor-pointer transition-colors"
                            onClick={() => handleSelectProviderModel(model.id)}
                          >
                            <div className="font-medium text-sm">{model.id}</div>
                            <div className="text-xs text-muted-foreground mt-1">
                              提供商: {group.provider.Name}
                              {model.owned_by ? ` · 归属: ${model.owned_by}` : ""}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
          
          <DialogFooter>
            <div className="flex items-center justify-between w-full">
              <span className="text-sm text-muted-foreground">
                共 {filteredProviderModels.length} 个模型
                {modelSearchQuery && providerModels.length !== filteredProviderModels.length &&
                  ` (已筛选，总计 ${providerModels.length} 个)`}
              </span>
              <Button type="button" variant="outline" onClick={() => setModelPickerOpen(false)}>
                取消
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
 
