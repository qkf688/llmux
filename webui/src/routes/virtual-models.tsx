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
  getModels,
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

  const fetchData = async () => {
    try {
      setLoading(true);
      const [vModels, rModels] = await Promise.all([
        getVirtualModels(),
        getModels(),
      ]);
      setVirtualModels(vModels);
      setRealModels(rModels);
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
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>管理映射 - {currentVirtualModel?.Name}</DialogTitle>
            <DialogDescription>
              配置虚拟模型关联的真实模型
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <Button onClick={handleAddMapping}>添加映射</Button>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>真实模型</TableHead>
                  <TableHead>优先级</TableHead>
                  <TableHead>权重</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mappings.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground">
                      暂无映射
                    </TableCell>
                  </TableRow>
                ) : (
                  mappings.map((mapping) => (
                    <TableRow key={mapping.ID}>
                      <TableCell>{getRealModelName(mapping.RealModelID)}</TableCell>
                      <TableCell>{mapping.Priority}</TableCell>
                      <TableCell>{mapping.Weight}</TableCell>
                      <TableCell>
                        <span className={`px-2 py-1 rounded text-xs ${mapping.Enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"}`}>
                          {mapping.Enabled ? "启用" : "禁用"}
                        </span>
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button variant="outline" size="sm" onClick={() => handleEditMapping(mapping)}>
                            编辑
                          </Button>
                          <Button variant="destructive" size="sm" onClick={() => handleDeleteMapping(mapping.ID)}>
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
    </div>
  );
}
