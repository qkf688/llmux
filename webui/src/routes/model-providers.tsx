import { useState, useEffect, useRef, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
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
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import Loading from "@/components/loading";
import {
  getModelProviders,
  getModelProviderStatus,
  getModelProviderHealthStatus,
  createModelProvider,
  updateModelProvider,
  updateModelProviderStatus,
  deleteModelProvider,
  batchDeleteModelProviders,
  getModels,
  getProviders,
  testModelProvider,
  getSettings
} from "@/lib/api";
import type { ModelWithProvider, Model, Provider, ProviderModel, Settings } from "@/lib/api";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { RefreshCw, ChevronDown, ChevronRight } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";

type MobileInfoItemProps = {
  label: string;
  value: ReactNode;
};

type ProviderModelWithOwner = ProviderModel & {
  providerId: number;
  providerName: string;
};

type ProviderModelSelection = {
  providerId: number;
  providerName: string;
  modelId: string;
};

type ProviderModelGroup = {
  provider: Provider;
  models: ProviderModelWithOwner[];
};

const buildSelectionKey = (providerId: number, modelId: string) =>
  `${providerId}::${modelId.toLowerCase()}`;

const MobileInfoItem = ({ label, value }: MobileInfoItemProps) => (
  <div className="space-y-1">
    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
    <div className="text-sm font-medium break-words">{value}</div>
  </div>
);

// 定义表单验证模式
const headerPairSchema = z.object({
  key: z.string().min(1, { message: "请求头键不能为空" }),
  value: z.string().default(""),
});

const formSchema = z.object({
  model_id: z.number().positive({ message: "模型ID必须大于0" }),
  provider_name: z.string().default(""),
  provider_id: z.number().min(0, { message: "提供商ID必须大于等于0" }),
  tool_call: z.boolean(),
  structured_output: z.boolean(),
  image: z.boolean(),
  with_header: z.boolean(),
  weight: z.number().positive({ message: "权重必须大于0" }),
  priority: z.number().min(0, { message: "优先级必须大于等于0" }),
  customer_headers: z.array(headerPairSchema).default([]),
});

type FormValues = z.input<typeof formSchema>;

export default function ModelProvidersPage() {
  const [modelProviders, setModelProviders] = useState<ModelWithProvider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const [providerStatus, setProviderStatus] = useState<Record<number, boolean[]>>({});
  const [healthStatus, setHealthStatus] = useState<Record<number, boolean[]>>({});
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editingAssociation, setEditingAssociation] = useState<ModelWithProvider | null>(null);
  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [testResults, setTestResults] = useState<Record<number, { loading: boolean; result: any }>>({});
  const [testDialogOpen, setTestDialogOpen] = useState(false);
  const [selectedTestId, setSelectedTestId] = useState<number | null>(null);
  const [testType, setTestType] = useState<"connectivity" | "react">("connectivity");
  const [selectedProviderType, setSelectedProviderType] = useState<string>("all");
  const [reactTestResult, setReactTestResult] = useState<{
    loading: boolean;
    messages: string;
    success: boolean | null;
    error: string | null;
  }>({
    loading: false,
    messages: "",
    success: null,
    error: null
  });
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [loadingProviderModels, setLoadingProviderModels] = useState(false);
  const [selectedProviderModels, setSelectedProviderModels] = useState<ProviderModelSelection[]>([]);
  const [modelListDialogOpen, setModelListDialogOpen] = useState(false);
  const [modelSearchKeyword, setModelSearchKeyword] = useState("");
  const [selectedAssociationIds, setSelectedAssociationIds] = useState<number[]>([]);
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [collapsedProviders, setCollapsedProviders] = useState<Record<number, boolean>>({});

  const dialogClose = () => {
    setTestDialogOpen(false)
  };

  // 初始化表单
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      model_id: 0,
      provider_name: "",
      provider_id: 0,
      tool_call: false,
      structured_output: false,
      image: false,
      with_header: false,
      weight: 1,
      priority: 100,
      customer_headers: [],
    },
  });
  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: "customer_headers",
  });

  useEffect(() => {
    Promise.all([fetchModels(), fetchProviders(), fetchSettings()]).finally(() => {
      setLoading(false);
    });
  }, []);

  useEffect(() => {
    if (models.length === 0) {
      if (selectedModelId !== null) {
        setSelectedModelId(null);
        form.setValue("model_id", 0);
      }
      return;
    }

    const modelIdParam = searchParams.get("modelId");
    const parsedParam = modelIdParam ? Number(modelIdParam) : NaN;

    if (!Number.isNaN(parsedParam) && models.some(model => model.ID === parsedParam)) {
      if (selectedModelId !== parsedParam) {
        setSelectedModelId(parsedParam);
        form.setValue("model_id", parsedParam);
      }
      return;
    }

    const fallbackId = models[0].ID;
    if (selectedModelId !== fallbackId) {
      setSelectedModelId(fallbackId);
      form.setValue("model_id", fallbackId);
    }
    if (modelIdParam !== fallbackId.toString()) {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set("modelId", fallbackId.toString());
      setSearchParams(nextParams, { replace: true });
    }
  }, [models, searchParams, form, setSearchParams]);

  useEffect(() => {
    if (selectedModelId) {
      fetchModelProviders(selectedModelId);
    }
  }, [selectedModelId]);

  const buildPayload = (
    values: FormValues,
    overrides?: {
      providerId?: number;
      providerModel?: string;
    }
  ) => {
    const headers: Record<string, string> = {};
    (values.customer_headers || []).forEach(({ key, value }) => {
      const trimmedKey = key.trim();
      if (trimmedKey) {
        headers[trimmedKey] = value ?? "";
      }
    });

    return {
      model_id: values.model_id,
      provider_name: (overrides?.providerModel ?? values.provider_name) || "",
      provider_id: overrides?.providerId ?? values.provider_id,
      tool_call: values.tool_call,
      structured_output: values.structured_output,
      image: values.image,
      with_header: values.with_header,
      customer_headers: headers,
      weight: values.weight,
      priority: values.priority
    };
  };

  const fetchModels = async () => {
    try {
      const data = await getModels();
      setModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型列表失败: ${message}`);
      console.error(err);
    }
  };

  const rebuildProviderModels = (providerList: Provider[]) => {
    setLoadingProviderModels(true);
    const groups = providerList.map((provider) => {
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
    setLoadingProviderModels(false);
  };

  const fetchProviders = async () => {
    try {
      const data = await getProviders();
      setProviders(data);
      rebuildProviderModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商列表失败: ${message}`);
      console.error(err);
    }
  };

  const fetchSettings = async () => {
    try {
      const data = await getSettings();
      setSettings(data);
    } catch (err) {
      console.error("获取系统设置失败", err);
    }
  };

  const fetchModelProviders = async (modelId: number) => {
    try {
      setLoading(true);
      const data = await getModelProviders(modelId);
      setModelProviders(data.map(item => ({
        ...item,
        CustomerHeaders: item.CustomerHeaders || {}
      })));
      // 异步加载状态数据
      loadProviderStatus(data, modelId);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型提供商关联列表失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const loadProviderStatus = async (providers: ModelWithProvider[], modelId: number) => {
    const selectedModel = models.find(m => m.ID === modelId);
    if (!selectedModel) return;
    setProviderStatus({});
    setHealthStatus({});

    const newStatus: Record<number, boolean[]> = {};
    const newHealthStatus: Record<number, boolean[]> = {};

    // 并行加载所有状态数据
    await Promise.all(
      providers.map(async (provider) => {
        try {
          const [status, healthStatusList] = await Promise.all([
            getModelProviderStatus(
              provider.ProviderID,
              selectedModel.Name,
              provider.ProviderModel
            ),
            getModelProviderHealthStatus(provider.ID)
          ]);
          newStatus[provider.ID] = status;
          newHealthStatus[provider.ID] = healthStatusList;
        } catch (error) {
          console.error(`Failed to load status for provider ${provider.ID}:`, error);
          newStatus[provider.ID] = [];
          newHealthStatus[provider.ID] = [];
        }
      })
    );

    setProviderStatus(newStatus);
    setHealthStatus(newHealthStatus);
  };

  const handleCreate = async (values: FormValues) => {
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      // 如果选择了多个模型，批量创建关联
      if (selectedProviderModels.length > 0) {
        const promises = selectedProviderModels.map(({ providerId, modelId }) =>
          createModelProvider(
            buildPayload(values, {
              providerId,
              providerModel: modelId
            })
          )
        );
        await Promise.all(promises);
        toast.success(`成功创建 ${selectedProviderModels.length} 个模型提供商关联`);
      } else {
        const modelName = values.provider_name?.trim();
        if (!modelName) {
          toast.error("请选择模型或手动输入模型名称");
          return;
        }
        if (!values.provider_id || values.provider_id <= 0) {
          toast.error("请选择具体的提供商");
          return;
        }
        await createModelProvider(
          buildPayload(values, {
            providerId: values.provider_id,
            providerModel: modelName
          })
        );
        toast.success("模型提供商关联创建成功");
      }
      
      setOpen(false);
      form.reset({
        model_id: selectedModelId || 0,
        provider_name: "",
        provider_id: 0,
        tool_call: false,
        structured_output: false,
        image: false,
        with_header: false,
        weight: 1,
        priority: 100,
        customer_headers: []
      });
      setSelectedProviderModels([]);
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建模型提供商关联失败: ${message}`);
      console.error(err);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleUpdate = async (values: FormValues) => {
    if (!editingAssociation) return;

    try {
      await updateModelProvider(editingAssociation.ID, buildPayload(values));
      setOpen(false);
      toast.success("模型提供商关联更新成功");
      setEditingAssociation(null);
      form.reset({
        model_id: 0,
        provider_name: "",
        provider_id: 0,
        tool_call: false,
        structured_output: false,
        image: false,
        with_header: false,
        weight: 1,
        priority: 100,
        customer_headers: []
      });
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新模型提供商关联失败: ${message}`);
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteModelProvider(deleteId);
      setDeleteId(null);
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
      toast.success("模型提供商关联删除成功");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除模型提供商关联失败: ${message}`);
      console.error(err);
    }
  };

  const handleStatusToggle = async (association: ModelWithProvider, nextStatus: boolean) => {
    const previousStatus = association.Status ?? true;
    setStatusError(null);
    setStatusUpdating(prev => ({ ...prev, [association.ID]: true }));
    setModelProviders(prev =>
      prev.map(item =>
        item.ID === association.ID ? { ...item, Status: nextStatus } : item
      )
    );

    try {
      const updated = await updateModelProviderStatus(association.ID, nextStatus);
      const normalized = { ...updated, CustomerHeaders: updated.CustomerHeaders || {} };
      setModelProviders(prev =>
        prev.map(item =>
          item.ID === association.ID ? normalized : item
        )
      );
    } catch (err) {
      setModelProviders(prev =>
        prev.map(item =>
          item.ID === association.ID ? { ...item, Status: previousStatus } : item
        )
      );
      setStatusError("更新启用状态失败");
      console.error(err);
    } finally {
      setStatusUpdating(prev => {
        const next = { ...prev };
        delete next[association.ID];
        return next;
      });
    }
  };

  const handleTest = (id: number) => {
    currentControllerRef.current?.abort(); // 取消之前的请求
    setSelectedTestId(id);
    setTestType("connectivity");
    setTestDialogOpen(true);
    setReactTestResult({
      loading: false,
      messages: "",
      success: null,
      error: null
    });
  };

  const handleConnectivityTest = async (id: number) => {
    try {
      setTestResults(prev => ({
        ...prev,
        [id]: { loading: true, result: null }
      }));

      const result = await testModelProvider(id);
      setTestResults(prev => ({
        ...prev,
        [id]: { loading: false, result }
      }));
      return result;
    } catch (err) {
      setTestResults(prev => ({
        ...prev,
        [id]: { loading: false, result: { error: "测试失败" + err } }
      }));
      console.error(err);
      return { error: "测试失败" + err };
    }
  };


  const currentControllerRef = useRef<AbortController | null>(null);
  const handleReactTest = async (id: number) => {
    setReactTestResult(prev => ({
      ...prev,
      messages: "",
      loading: true,
    }));
    try {
      const token = localStorage.getItem("authToken");
      const controller = new AbortController();
      currentControllerRef.current = controller;
      await fetchEventSource(`/api/test/react/${id}`, {
        method: "GET",
        headers: {
          "Authorization": `Bearer ${token}`,
        },
        signal: controller.signal,
        onmessage(event) {
          setReactTestResult(prev => {
            if (event.event === "start") {
              return {
                ...prev,
                messages: prev.messages + `[开始测试] ${event.data}\n`
              };
            } else if (event.event === "toolcall") {
              return {
                ...prev,
                messages: prev.messages + `\n[调用工具] ${event.data}\n`
              };
            } else if (event.event === "toolres") {
              return {
                ...prev,
                messages: prev.messages + `\n[工具输出] ${event.data}\n`
              };
            }
            else if (event.event === "message") {
              if (event.data.trim()) {
                return {
                  ...prev,
                  messages: prev.messages + `${event.data}`
                };
              }
            } else if (event.event === "error") {
              return {
                ...prev,
                success: false,
                messages: prev.messages + `\n[错误] ${event.data}\n`
              };
            } else if (event.event === "success") {
              return {
                ...prev,
                success: true,
                messages: prev.messages + `\n[成功] ${event.data}`
              };
            }
            return prev;
          });
        },
        onclose() {
          setReactTestResult(prev => {
            return {
              ...prev,
              loading: false,
            };
          });
        },
        onerror(err) {
          setReactTestResult(prev => {
            return {
              ...prev,
              loading: false,
              error: err.message || "测试过程中发生错误",
              success: false
            };
          });
          throw err;
        }
      });
    } catch (err) {
      setReactTestResult(prev => ({
        ...prev,
        loading: false,
        error: "测试失败",
        success: false
      }));
      console.error(err);
    }
  };

  const executeTest = async () => {
    if (!selectedTestId) return;

    if (testType === "connectivity") {
      await handleConnectivityTest(selectedTestId);
    } else {
      await handleReactTest(selectedTestId);
    }
  };

  const openEditDialog = (association: ModelWithProvider) => {
    setEditingAssociation(association);
    setSelectedProviderModels([]);
    const headerPairs = Object.entries(association.CustomerHeaders || {}).map(([key, value]) => ({
      key,
      value,
    }));
    form.reset({
      model_id: association.ModelID,
      provider_name: association.ProviderModel,
      provider_id: association.ProviderID,
      tool_call: association.ToolCall,
      structured_output: association.StructuredOutput,
      image: association.Image,
      with_header: association.WithHeader,
      weight: association.Weight,
      priority: association.Priority ?? 100,
      customer_headers: headerPairs.length ? headerPairs : [],
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingAssociation(null);
    setSelectedProviderModels([]);
    // 如果开启了自动权重衰减，使用设置中的默认权重值
    const defaultWeight = settings?.auto_weight_decay ? (settings.auto_weight_decay_default || 100) : 1;
    form.reset({
      model_id: selectedModelId || 0,
      provider_name: "",
      provider_id: 0,
      tool_call: false,
      structured_output: false,
      image: false,
      with_header: false,
      weight: defaultWeight,
      priority: 100,
      customer_headers: []
    });
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const handleSelectAllAssociations = (checked: boolean) => {
    if (checked) {
      setSelectedAssociationIds(filteredModelProviders.map(mp => mp.ID));
    } else {
      setSelectedAssociationIds([]);
    }
  };

  const handleSelectOneAssociation = (id: number, checked: boolean) => {
    if (checked) {
      setSelectedAssociationIds([...selectedAssociationIds, id]);
    } else {
      setSelectedAssociationIds(selectedAssociationIds.filter(selectedId => selectedId !== id));
    }
  };

  const handleBatchDeleteAssociations = async () => {
    if (selectedAssociationIds.length === 0) return;
    setBatchDeleting(true);
    try {
      const result = await batchDeleteModelProviders(selectedAssociationIds);
      toast.success(`成功删除 ${result.deleted} 个关联`);
      setSelectedAssociationIds([]);
      setBatchDeleteDialogOpen(false);
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`批量删除关联失败: ${message}`);
    } finally {
      setBatchDeleting(false);
    }
  };

  const handleModelChange = (modelId: string) => {
    const id = parseInt(modelId);
    setSelectedModelId(id);
    setSelectedAssociationIds([]); // 切换模型时清空选择
    setSelectedProviderModels([]);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set("modelId", id.toString());
    setSearchParams(nextParams);
    form.setValue("model_id", id);
  };

  const toggleProviderCollapse = (providerId: number) => {
    setCollapsedProviders((prev) => ({
      ...prev,
      [providerId]: !prev[providerId],
    }));
  };

  // 获取唯一的提供商类型列表
  const providerTypes = Array.from(new Set(providers.map(p => p.Type).filter(Boolean)));

  // 根据选择的提供商类型过滤模型提供商关联
  const filteredModelProviders = selectedProviderType && selectedProviderType !== "all"
    ? modelProviders.filter(association => {
      const provider = providers.find(p => p.ID === association.ProviderID);
      return provider?.Type === selectedProviderType;
    })
    : modelProviders;

  const hasAssociationFilter = selectedProviderType !== "all";

  const isAllAssociationsSelected = filteredModelProviders.length > 0 && selectedAssociationIds.length === filteredModelProviders.length;
  const isPartialAssociationsSelected = selectedAssociationIds.length > 0 && selectedAssociationIds.length < filteredModelProviders.length;

  const existingAssociationKeys = new Set(
    modelProviders.map((mp) => buildSelectionKey(mp.ProviderID, mp.ProviderModel))
  );
  const searchKeywordLower = modelSearchKeyword.toLowerCase();
  const selectedProviderId = useWatch({
    control: form.control,
    name: "provider_id"
  });
  const visibleProviderGroups = providerModelGroups
    .filter((group) =>
      selectedProviderId && selectedProviderId > 0 ? group.provider.ID === selectedProviderId : true
    )
    .map((group) => ({
      ...group,
      models: group.models.filter((model) =>
        model.id.toLowerCase().includes(searchKeywordLower)
      )
    }))
    .filter((group) => group.models.length > 0);
  const visibleProviderModels = visibleProviderGroups.flatMap((group) => group.models);
  const visibleAvailableModels = visibleProviderModels.filter(
    (model) => !existingAssociationKeys.has(buildSelectionKey(model.providerId, model.id))
  );
  const visibleExistingCount = visibleProviderModels.length - visibleAvailableModels.length;
  const selectedKeys = new Set(
    selectedProviderModels.map((item) => buildSelectionKey(item.providerId, item.modelId))
  );

  if (loading && models.length === 0 && providers.length === 0) return <Loading message="加载模型和提供商" />;

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">模型提供商关联</h2>
          </div>
          <div className="flex w-full sm:w-auto items-center justify-end gap-2">
          </div>
        </div>
      </div>
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:gap-4">
          <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">关联模型</Label>
            <Select value={selectedModelId?.toString() || ""} onValueChange={handleModelChange}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="选择模型" />
              </SelectTrigger>
              <SelectContent>
                {models.map((model) => (
                  <SelectItem key={model.ID} value={model.ID.toString()}>
                    {model.Name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商类型</Label>
            <Select value={selectedProviderType} onValueChange={setSelectedProviderType}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="按类型筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {providerTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-end col-span-2 sm:col-span-1 sm:justify-end gap-2">
            {selectedAssociationIds.length > 0 && (
              <AlertDialog open={batchDeleteDialogOpen} onOpenChange={setBatchDeleteDialogOpen}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" className="h-8 text-xs">
                    批量删除 ({selectedAssociationIds.length})
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>确定要批量删除这些关联吗？</AlertDialogTitle>
                    <AlertDialogDescription>
                      此操作无法撤销。这将永久删除选中的 {selectedAssociationIds.length} 个模型提供商关联。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={batchDeleting}>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={handleBatchDeleteAssociations} disabled={batchDeleting}>
                      {batchDeleting ? "删除中..." : "确认删除"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
            <Button
              onClick={openCreateDialog}
              disabled={!selectedModelId}
              className="h-8 w-full text-xs sm:w-auto sm:ml-auto"
            >
              添加关联
            </Button>
          </div>
        </div>
      </div>
      {statusError && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {statusError}
        </div>
      )}
      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载关联数据" />
          </div>
        ) : !selectedModelId ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            请选择一个模型来查看其提供商关联
          </div>
        ) : filteredModelProviders.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm text-center px-6">
            {hasAssociationFilter ? '当前类型暂无关联' : '该模型还没有关联的提供商'}
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="hidden sm:block w-full overflow-x-auto">
              <Table className="min-w-[1150px]">
                <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
                  <TableRow>
                    <TableHead className="w-[50px]">
                      <Checkbox
                        checked={isAllAssociationsSelected}
                        ref={(el) => {
                          if (el) {
                            (el as unknown as HTMLInputElement).indeterminate = isPartialAssociationsSelected;
                          }
                        }}
                        onCheckedChange={handleSelectAllAssociations}
                        aria-label="全选"
                      />
                    </TableHead>
                    <TableHead>ID</TableHead>
                    <TableHead>提供商模型</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>提供商</TableHead>
                    <TableHead>工具调用</TableHead>
                    <TableHead>结构化输出</TableHead>
                    <TableHead>视觉</TableHead>
                    <TableHead>请求头透传</TableHead>
                    <TableHead>权重</TableHead>
                    <TableHead>优先级</TableHead>
                    <TableHead>启用</TableHead>
                    <TableHead>
                      <div className="flex items-center gap-1">状态
                        <Button
                          onClick={() => loadProviderStatus(modelProviders, selectedModelId)}
                          variant="ghost"
                          size="icon"
                          aria-label="刷新状态"
                          title="刷新状态"
                          className="rounded-full"
                        >
                          <RefreshCw className="size-4" />
                        </Button>
                      </div>
                    </TableHead>
                    <TableHead>健康检测</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredModelProviders.map((association) => {
                    const provider = providers.find(p => p.ID === association.ProviderID);
                    const isAssociationEnabled = association.Status ?? false;
                    const statusBars = providerStatus[association.ID];
                    const healthBars = healthStatus[association.ID];
                    return (
                      <TableRow key={association.ID}>
                        <TableCell>
                          <Checkbox
                            checked={selectedAssociationIds.includes(association.ID)}
                            onCheckedChange={(checked) => handleSelectOneAssociation(association.ID, !!checked)}
                            aria-label={`选择 ${association.ProviderModel}`}
                          />
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">{association.ID}</TableCell>
                        <TableCell className="max-w-[200px] truncate" title={association.ProviderModel}>
                          {association.ProviderModel}
                        </TableCell>
                        <TableCell>{provider?.Type ?? '未知'}</TableCell>
                        <TableCell>{provider?.Name ?? '未知'}</TableCell>
                        <TableCell>
                          <span className={association.ToolCall ? "text-green-600" : "text-red-600"}>
                            {association.ToolCall ? '✓' : '✗'}
                          </span>
                        </TableCell>
                        <TableCell>
                          <span className={association.StructuredOutput ? "text-green-600" : "text-red-600"}>
                            {association.StructuredOutput ? '✓' : '✗'}
                          </span>
                        </TableCell>
                        <TableCell>
                          <span className={association.Image ? "text-green-600" : "text-red-600"}>
                            {association.Image ? '✓' : '✗'}
                          </span>
                        </TableCell>
                        <TableCell>
                          <span className={association.WithHeader ? "text-green-600" : "text-red-600"}>
                            {association.WithHeader ? '✓' : '✗'}
                          </span>
                        </TableCell>
                        <TableCell>{association.Weight}</TableCell>
                        <TableCell>{association.Priority ?? 100}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Switch
                              checked={isAssociationEnabled}
                              disabled={!!statusUpdating[association.ID]}
                              onCheckedChange={(value) => handleStatusToggle(association, value)}
                              aria-label="切换启用状态"
                            />
                            <span className="text-xs text-muted-foreground">
                              {isAssociationEnabled ? '已启用' : '已停用'}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center space-x-4 w-20">
                            {statusBars ? (
                              statusBars.length > 0 ? (
                                <div className="flex space-x-1 items-end h-6">
                                  {statusBars.map((isSuccess, index) => (
                                    <div
                                      key={index}
                                      className={`w-1 h-6 ${isSuccess ? 'bg-green-500' : 'bg-red-500'}`}
                                      title={isSuccess ? '成功' : '失败'}
                                    />
                                  ))}
                                </div>
                              ) : (
                                <div className="text-xs text-gray-400">无数据</div>
                              )
                            ) : (
                              <Spinner />
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center space-x-4 w-20">
                            {healthBars ? (
                              healthBars.length > 0 ? (
                                <div className="flex space-x-1 items-end h-6">
                                  {healthBars.map((isSuccess, index) => (
                                    <div
                                      key={index}
                                      className={`w-1 h-6 ${isSuccess ? 'bg-emerald-500' : 'bg-orange-500'}`}
                                      title={isSuccess ? '健康检测成功' : '健康检测失败'}
                                    />
                                  ))}
                                </div>
                              ) : (
                                <div className="text-xs text-gray-400">无数据</div>
                              )
                            ) : (
                              <Spinner />
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => openEditDialog(association)}>
                              编辑
                            </Button>
                            <AlertDialog open={deleteId === association.ID} onOpenChange={(open) => !open && setDeleteId(null)}>
                              <Button variant="destructive" size="sm" onClick={() => openDeleteDialog(association.ID)}>
                                删除
                              </Button>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>确定要删除这个关联吗？</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    此操作无法撤销。这将永久删除该模型提供商关联。
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                                  <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
                            <Button variant="outline" size="sm" onClick={() => handleTest(association.ID)}>
                              测试
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
            <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-3 divide-y divide-border">
              {/* 移动端全选 */}
              <div className="py-2 flex items-center gap-2 border-b">
                <Checkbox
                  checked={isAllAssociationsSelected}
                  ref={(el) => {
                    if (el) {
                      (el as unknown as HTMLInputElement).indeterminate = isPartialAssociationsSelected;
                    }
                  }}
                  onCheckedChange={handleSelectAllAssociations}
                  aria-label="全选"
                />
                <span className="text-sm text-muted-foreground">
                  {selectedAssociationIds.length > 0 ? `已选择 ${selectedAssociationIds.length} 项` : "全选"}
                </span>
              </div>
              {filteredModelProviders.map((association) => {
                const provider = providers.find(p => p.ID === association.ProviderID);
                const isAssociationEnabled = association.Status ?? true;
                const statusBars = providerStatus[association.ID];
                const healthBars = healthStatus[association.ID];
                return (
                  <div key={association.ID} className="py-3 space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2 min-w-0 flex-1">
                        <Checkbox
                          checked={selectedAssociationIds.includes(association.ID)}
                          onCheckedChange={(checked) => handleSelectOneAssociation(association.ID, !!checked)}
                          aria-label={`选择 ${association.ProviderModel}`}
                        />
                        <div className="min-w-0 flex-1">
                          <h3 className="font-semibold text-sm truncate">{provider?.Name ?? '未知提供商'}</h3>
                          <p className="text-[11px] text-muted-foreground">提供商模型: {association.ProviderModel}</p>
                        </div>
                      </div>
                      <span
                        className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${isAssociationEnabled ? 'bg-emerald-100 text-emerald-700' : 'bg-red-100 text-red-700'}`}
                      >
                        {isAssociationEnabled ? '已启用' : '已停用'}
                      </span>
                    </div>
                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <MobileInfoItem label="提供商类型" value={provider?.Type ?? '未知'} />
                      <MobileInfoItem label="提供商 ID" value={<span className="font-mono text-xs">{provider?.ID ?? '-'}</span>} />
                      <MobileInfoItem label="权重" value={association.Weight} />
                      <MobileInfoItem label="优先级" value={association.Priority ?? 100} />
                    </div>
                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <MobileInfoItem
                        label="请求头透传"
                        value={<span className={association.WithHeader ? "text-green-600" : "text-red-600"}>{association.WithHeader ? '✓' : '✗'}</span>}
                      />
                      <MobileInfoItem
                        label="工具调用"
                        value={<span className={association.ToolCall ? "text-green-600" : "text-red-600"}>{association.ToolCall ? '✓' : '✗'}</span>}
                      />
                      <MobileInfoItem
                        label="结构化输出"
                        value={<span className={association.StructuredOutput ? "text-green-600" : "text-red-600"}>{association.StructuredOutput ? '✓' : '✗'}</span>}
                      />
                      <MobileInfoItem
                        label="视觉能力"
                        value={<span className={association.Image ? "text-green-600" : "text-red-600"}>{association.Image ? '✓' : '✗'}</span>}
                      />
                      <MobileInfoItem
                        label="最近状态"
                        value={
                          <div className="flex items-center gap-1">
                            {statusBars ? (
                              statusBars.length > 0 ? (
                                statusBars.map((isSuccess, index) => (
                                  <div
                                    key={index}
                                    className={`w-1 h-4 rounded ${isSuccess ? 'bg-green-500' : 'bg-red-500'}`}
                                  />
                                ))
                              ) : (
                                <span className="text-muted-foreground text-[11px]">无数据</span>
                              )
                            ) : (
                              <Spinner />
                            )}
                          </div>
                        }
                      />
                      <MobileInfoItem
                        label="健康检测"
                        value={
                          <div className="flex items-center gap-1">
                            {healthBars ? (
                              healthBars.length > 0 ? (
                                healthBars.map((isSuccess, index) => (
                                  <div
                                    key={index}
                                    className={`w-1 h-4 rounded ${isSuccess ? 'bg-emerald-500' : 'bg-orange-500'}`}
                                  />
                                ))
                              ) : (
                                <span className="text-muted-foreground text-[11px]">无数据</span>
                              )
                            ) : (
                              <Spinner />
                            )}
                          </div>
                        }
                      />
                    </div>
                    <div className="flex items-center justify-between rounded-md border bg-muted/30 px-3 py-2">
                      <p className="text-xs text-muted-foreground">启用状态</p>
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{isAssociationEnabled ? "启用" : "停用"}</span>
                        <Switch
                          checked={isAssociationEnabled}
                          disabled={!!statusUpdating[association.ID]}
                          onCheckedChange={(value) => handleStatusToggle(association, value)}
                          aria-label="切换启用状态"
                        />
                      </div>
                    </div>
                    <div className="flex flex-wrap justify-end gap-1.5">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2 text-xs"
                        onClick={() => openEditDialog(association)}
                      >
                        编辑
                      </Button>
                      <AlertDialog open={deleteId === association.ID} onOpenChange={(open) => !open && setDeleteId(null)}>
                        <Button
                          variant="destructive"
                          size="sm"
                          className="h-7 px-2 text-xs"
                          onClick={() => openDeleteDialog(association.ID)}
                        >
                          删除
                        </Button>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>确定要删除这个关联吗？</AlertDialogTitle>
                            <AlertDialogDescription>
                              此操作无法撤销。这将永久删除该模型提供商关联。
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                            <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2 text-xs"
                        onClick={() => handleTest(association.ID)}
                      >
                        测试
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>
              {editingAssociation ? "编辑关联" : "添加关联"}
            </DialogTitle>
            <DialogDescription>
              {editingAssociation
                ? "修改模型提供商关联"
                : "添加一个新的模型提供商关联"}
            </DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(editingAssociation ? handleUpdate : handleCreate)} className="flex flex-col gap-4 flex-1 min-h-0">
              <div className="space-y-4 overflow-y-auto pr-1 sm:pr-2 max-h-[60vh] flex-1 min-h-0">
                <FormField
                  control={form.control}
                  name="model_id"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>模型</FormLabel>
                      <Select
                        value={field.value.toString()}
                        onValueChange={(value) => field.onChange(parseInt(value))}
                        disabled={!!editingAssociation}
                      >
                        <FormControl>
                          <SelectTrigger className="form-select">
                            <SelectValue placeholder="选择模型" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {models.map((model) => (
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

                <FormField
                  control={form.control}
                  name="provider_id"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>提供商（默认全部）</FormLabel>
                      <Select
                        value={field.value && field.value > 0 ? field.value.toString() : "0"}
                        onValueChange={(value) => {
                          if (value === "0") {
                            field.onChange(0);
                            setSelectedProviderModels([]);
                            return;
                          }
                          const providerId = parseInt(value, 10);
                          field.onChange(providerId);
                          setSelectedProviderModels([]);
                        }}
                      >
                        <FormControl>
                          <SelectTrigger className="form-select">
                            <SelectValue placeholder="选择提供商" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="0">全部</SelectItem>
                          {providers.map((provider) => (
                            <SelectItem key={provider.ID} value={provider.ID.toString()}>
                              {provider.Name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormItem>
                  <FormLabel>提供商模型</FormLabel>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-muted-foreground">
                        已选择 {selectedProviderModels.length} 个模型
                      </span>
                      <div className="flex gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setModelSearchKeyword("");
                            setModelListDialogOpen(true);
                          }}
                          disabled={providers.length === 0}
                        >
                          模型列表
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => setSelectedProviderModels([])}
                          disabled={selectedProviderModels.length === 0}
                        >
                          清空
                        </Button>
                      </div>
                    </div>
                    {selectedProviderModels.length > 0 && (
                      <div className="max-h-32 overflow-y-auto border rounded-md p-2 space-y-1">
                        {selectedProviderModels.map((selection) => {
                          const selectionKey = buildSelectionKey(selection.providerId, selection.modelId);
                          return (
                            <div key={selectionKey} className="flex items-center justify-between text-sm py-1 px-2 bg-muted/50 rounded">
                              <span className="truncate">{selection.providerName} / {selection.modelId}</span>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-5 w-5 p-0"
                              onClick={() =>
                                setSelectedProviderModels((prev) =>
                                  prev.filter(
                                    (item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey
                                  )
                                )
                              }
                            >
                              ✕
                            </Button>
                          </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                </FormItem>

                {/* 单个模型输入框，当未选择任何模型时显示 */}
                {selectedProviderModels.length === 0 && (
                  <FormField
                    control={form.control}
                    name="provider_name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>手动输入模型名称</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder="输入提供商模型名称"
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormLabel>模型能力</FormLabel>
                <FormField
                  control={form.control}
                  name="tool_call"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>
                          工具调用
                        </FormLabel>
                      </div>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="structured_output"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>
                          结构化输出
                        </FormLabel>
                      </div>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="image"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>
                          视觉
                        </FormLabel>
                      </div>
                    </FormItem>
                  )}
                />
                <FormLabel>参数配置</FormLabel>
                <FormField
                  control={form.control}
                  name="with_header"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>
                          请求头透传
                        </FormLabel>
                      </div>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="customer_headers"
                  render={({ field }) => {
                    const headerValues = field.value ?? [];
                    return (
                      <FormItem>
                        <div className="flex items-center justify-between">
                          <FormLabel>自定义请求头</FormLabel>
                          <Button type="button" variant="outline" size="sm" onClick={() => appendHeader({ key: "", value: "" })}>
                            添加
                          </Button>
                        </div>
                        <div className="space-y-2">
                          {headerFields.map((header, index) => {
                            const errorMsg = form.formState.errors.customer_headers?.[index]?.key?.message;
                            return (
                              <div key={header.id} className="space-y-1">
                                <div className="flex gap-2 items-center">
                                  <div className="flex-1">
                                    <Input
                                      placeholder="Header Key"
                                      value={headerValues[index]?.key ?? ""}
                                      onChange={(e) => {
                                        const next = [...headerValues];
                                        next[index] = { ...next[index], key: e.target.value };
                                        field.onChange(next);
                                      }}
                                    />
                                  </div>
                                  <div className="flex-1">
                                    <Input
                                      placeholder="Header Value"
                                      value={headerValues[index]?.value ?? ""}
                                      onChange={(e) => {
                                        const next = [...headerValues];
                                        next[index] = { ...next[index], value: e.target.value };
                                        field.onChange(next);
                                      }}
                                    />
                                  </div>
                                  <Button type="button" size="sm" variant="destructive" onClick={() => removeHeader(index)}>
                                    删除
                                  </Button>
                                </div>
                                {errorMsg && (
                                  <p className="text-sm text-red-500">
                                    {errorMsg}
                                  </p>
                                )}
                              </div>
                            );
                          })}
                          <p className="text-sm text-muted-foreground">
                            {"优先级: 提供商配置 > 自定义请求头 > 透传请求头"}
                          </p>
                        </div>
                      </FormItem>
                    );
                  }}
                />

                <FormField
                  control={form.control}
                  name="weight"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>权重 (必须大于0)</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type="number"
                          min="1"
                          onChange={(e) => field.onChange(parseInt(e.target.value) || 0)}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="priority"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>优先级 (优先级高的优先选择，相同优先级按权重随机)</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type="number"
                          min="0"
                          onChange={(e) => field.onChange(parseInt(e.target.value) || 0)}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={isSubmitting}>
                  取消
                </Button>
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "提交中..." : (editingAssociation ? "更新" : "创建")}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* Test Dialog */}
      <Dialog open={testDialogOpen} onOpenChange={setTestDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>模型测试</DialogTitle>
            <DialogDescription>
              选择要执行的测试类型
            </DialogDescription>
          </DialogHeader>

          <RadioGroup value={testType} onValueChange={(value: string) => setTestType(value as "connectivity" | "react")} className="space-y-4">
            <div className="flex items-center space-x-2">
              <RadioGroupItem value="connectivity" id="connectivity" />
              <Label htmlFor="connectivity">连通性测试</Label>
            </div>
            <p className="text-sm text-gray-500 ml-6">测试模型提供商的基本连通性</p>

            <div className="flex items-center space-x-2">
              <RadioGroupItem value="react" id="react" />
              <Label htmlFor="react">React Agent 能力测试</Label>
            </div>
            <p className="text-sm text-gray-500 ml-6">测试模型的工具调用和反应能力</p>
          </RadioGroup>

          {testType === "connectivity" && (
            <div className="mt-4">
              {selectedTestId && testResults[selectedTestId]?.loading ? (
                <div className="flex items-center justify-center py-4">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
                  <span className="ml-2">测试中...</span>
                </div>
              ) : selectedTestId && testResults[selectedTestId] ? (
                <div className={`p-4 rounded-md ${testResults[selectedTestId].result?.error ? "bg-red-100 text-red-800" : "bg-green-100 text-green-800"}`}>
                  <p>{testResults[selectedTestId].result?.error ? testResults[selectedTestId].result?.error : "测试成功"}</p>
                  {testResults[selectedTestId].result?.message && (
                    <p className="mt-2">{testResults[selectedTestId].result.message}</p>
                  )}
                </div>
              ) : (
                <p className="text-gray-500">点击"执行测试"开始测试</p>
              )}
            </div>
          )}

          {testType === "react" && (
            <div className="mt-4 max-h-96 min-w-0">
              {reactTestResult.loading ? (
                <div className="flex items-center justify-center py-4">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
                  <span className="ml-2">测试中...</span>
                </div>
              ) : (
                <>
                  {reactTestResult.error ? (
                    <div className="p-4 rounded-md bg-red-100 text-red-800">
                      <p>测试失败: {reactTestResult.error}</p>
                    </div>
                  ) : reactTestResult.success !== null ? (
                    <div className={`p-4 rounded-md ${reactTestResult.success ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800"}`}>
                      <p>{reactTestResult.success ? "测试成功！" : "测试失败"}</p>
                    </div>
                  ) : null}


                </>
              )}

              {reactTestResult.messages && <Textarea name="logs" className="mt-4 max-h-50 resize-none whitespace-pre overflow-x-auto" readOnly value={reactTestResult.messages}>
              </Textarea>}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={dialogClose}>
              关闭
            </Button>
            <Button onClick={executeTest} disabled={testType === "connectivity" ?
              (selectedTestId ? testResults[selectedTestId]?.loading : false) :
              reactTestResult.loading}>
              {testType === "connectivity" ?
                (selectedTestId && testResults[selectedTestId]?.loading ? "测试中..." : "执行测试") :
                (reactTestResult.loading ? "测试中..." : "执行测试")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Model List Selection Dialog */}
      <Dialog open={modelListDialogOpen} onOpenChange={setModelListDialogOpen}>
        <DialogContent className="max-w-lg max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>选择模型</DialogTitle>
            <DialogDescription>
            从提供商的全部模型缓存中选择要关联的模型
            </DialogDescription>
          </DialogHeader>
          
          <div className="space-y-4 flex-1 min-h-0 flex flex-col">
            {/* 搜索框 */}
            <div className="flex-shrink-0">
              <Input
                placeholder="搜索模型..."
                value={modelSearchKeyword}
                onChange={(e) => setModelSearchKeyword(e.target.value)}
                className="w-full"
              />
            </div>
            
            {/* 操作按钮 */}
            <div className="flex items-center justify-between flex-shrink-0">
              <span className="text-sm text-muted-foreground">
                {loadingProviderModels
                  ? "加载中..."
                  : `共 ${visibleAvailableModels.length} 个可选模型${visibleExistingCount > 0 ? `（${visibleExistingCount} 个已关联）` : ""}`}
              </span>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setSelectedProviderModels((prev) => {
                      const merged = new Map(
                        prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item])
                      );
                      visibleAvailableModels.forEach((model) => {
                        merged.set(buildSelectionKey(model.providerId, model.id), {
                          providerId: model.providerId,
                          providerName: model.providerName,
                          modelId: model.id,
                        });
                      });
                      return Array.from(merged.values());
                    });
                  }}
                  disabled={loadingProviderModels || visibleAvailableModels.length === 0}
                >
                  全选可选
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setSelectedProviderModels([])}
                  disabled={selectedProviderModels.length === 0}
                >
                  清空
                </Button>
              </div>
            </div>
            
            {/* 模型列表 */}
            <div className="flex-1 min-h-0 overflow-y-auto border rounded-md p-2 space-y-2">
              {loadingProviderModels ? (
                <div className="flex items-center justify-center py-4">
                  <Spinner className="h-4 w-4" />
                  <span className="ml-2 text-sm">加载全部模型...</span>
                </div>
              ) : providerModels.length === 0 ? (
                <div className="text-center py-4 text-sm text-muted-foreground">
                  暂无全部模型缓存，请先在提供商管理页同步
                </div>
              ) : visibleProviderGroups.length === 0 ? (
                <div className="text-center py-4 text-sm text-muted-foreground">
                  没有匹配的模型
                </div>
              ) : (
                visibleProviderGroups.map((group) => {
                  const isCollapsed = collapsedProviders[group.provider.ID] ?? false;
                  return (
                    <div key={group.provider.ID} className="rounded-md border">
                      <button
                        type="button"
                        className="flex w-full items-center justify-between px-2 py-2 text-left hover:bg-muted/70 transition"
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
                          {group.models.map((model) => {
                            const selectionKey = buildSelectionKey(model.providerId, model.id);
                            const isExisting = existingAssociationKeys.has(selectionKey);
                            const checked = selectedKeys.has(selectionKey);
                            return (
                              <div
                                key={selectionKey}
                                className={`flex items-center gap-2 px-3 py-2 ${isExisting ? "opacity-60" : ""}`}
                              >
                                <Checkbox
                                  id={`dialog-model-${selectionKey}`}
                                  checked={checked}
                                  disabled={isExisting}
                                  onCheckedChange={(checkedValue) => {
                                    if (checkedValue) {
                                      setSelectedProviderModels((prev) => {
                                        const merged = new Map(
                                          prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item])
                                        );
                                        merged.set(selectionKey, {
                                          providerId: model.providerId,
                                          providerName: model.providerName,
                                          modelId: model.id,
                                        });
                                        return Array.from(merged.values());
                                      });
                                    } else {
                                      setSelectedProviderModels((prev) =>
                                        prev.filter(
                                          (item) =>
                                            buildSelectionKey(item.providerId, item.modelId) !== selectionKey
                                        )
                                      );
                                    }
                                  }}
                                />
                                <div className="min-w-0 flex-1">
                                  <Label
                                    htmlFor={`dialog-model-${selectionKey}`}
                                    className={`text-sm cursor-pointer truncate ${isExisting ? "cursor-not-allowed" : ""}`}
                                  >
                                    {model.id}
                                    {isExisting && (
                                      <span className="ml-2 text-xs text-muted-foreground">（已关联）</span>
                                    )}
                                  </Label>
                                  <p className="text-xs text-muted-foreground">提供商：{model.providerName}</p>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })
              )}
            </div>
            
            <div className="text-xs text-muted-foreground flex-shrink-0">
              已选择 {selectedProviderModels.length} 个模型
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setModelListDialogOpen(false)}>
              确定
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
 
