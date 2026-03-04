import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import Loading from "@/components/loading";
import {
  addModelTemplateItem,
  autoAssociateModels,
  batchDeleteModelProviders,
  batchUpdateModelProvidersStatus,
  cleanInvalidAssociations,
  createModelProvider,
  deleteModelProvider,
  deleteModelTemplateItem,
  enableAllAssociations,
  getModelProviderHealthStatus,
  getModelProviderStatus,
  getModelProviders,
  getModelTemplate,
  getModels,
  getProviderBlacklist,
  getProviders,
  getSettings,
  previewAutoAssociate,
  previewCleanInvalid,
  resetModelPriorities,
  resetModelWeights,
  testModelProvider,
  testModelProviderStructuredOutput,
  updateModelProvider,
  updateModelProviderStatus,
  updateProviderBlacklist,
} from "@/lib/api";
import type {
  AssociationPreview,
  Model,
  ModelProviderTestResult,
  ModelTemplate,
  ModelWithProvider,
  Provider,
  Settings,
} from "@/lib/api";
import { parseAllModelsFromConfig, toProviderModelList } from "@/lib/provider-models";
import { toast } from "sonner";
import { formSchema, type FormValues } from "./form-schema";
import type {
  AssociationBatchTestResult,
  BatchTestProgress,
  BlacklistFilter,
  ProviderModelGroup,
  ProviderModelSelection,
  ProviderModelWithOwner,
  TestType,
} from "./types";
import { buildAssociationPayload } from "./utils/payload";
import { buildSelectionKey } from "./utils/selection";
import { BlacklistDialog } from "./components/dialogs/blacklist-dialog";
import { ModelListDialog } from "./components/dialogs/model-list-dialog";
import { PreviewDialog } from "./components/dialogs/preview-dialog";
import { TemplateEditorDialog } from "./components/dialogs/template-editor-dialog";
import { TestDialog } from "./components/dialogs/test-dialog";
import { AssociationFormDialog } from "./components/dialogs/association-form-dialog";
import { AssociationFilterPanel } from "./components/sections/association-filter-panel";
import { BatchTestProgressCard } from "./components/sections/batch-test-progress-card";
import { OperationScopeToolbar } from "./components/sections/operation-scope-toolbar";
import { AssociationListSection } from "./components/sections/associations/association-list-section";
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
  const [testResults, setTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [structuredTestResults, setStructuredTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [testDialogOpen, setTestDialogOpen] = useState(false);
  const [selectedTestId, setSelectedTestId] = useState<number | null>(null);
  const [testType, setTestType] = useState<TestType>("connectivity");
  const [selectedProviderType, setSelectedProviderType] = useState<string>("all");
  const [selectedProviderFilter, setSelectedProviderFilter] = useState<string>("all");
  const [selectedStatusFilter, setSelectedStatusFilter] = useState<string>("all");
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
  const [batchUpdatingStatus, setBatchUpdatingStatus] = useState(false);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [collapsedProviders, setCollapsedProviders] = useState<Record<number, boolean>>({});
  const [searchKeyword, setSearchKeyword] = useState("");
  const [previewDialogOpen, setPreviewDialogOpen] = useState(false);
  const [previewData, setPreviewData] = useState<AssociationPreview[]>([]);
  const [previewType, setPreviewType] = useState<"associate" | "clean">("associate");
  const [executing, setExecuting] = useState(false);
  const [templateEditorOpen, setTemplateEditorOpen] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);
  const [templateData, setTemplateData] = useState<ModelTemplate | null>(null);
  const [templateNewItem, setTemplateNewItem] = useState("");
  const [resettingWeights, setResettingWeights] = useState(false);
  const [resettingPriorities, setResettingPriorities] = useState(false);
  const [enablingAssociations, setEnablingAssociations] = useState(false);
  const [operationScope, setOperationScope] = useState<"current" | "all">("current");
  
  // 筛选面板折叠状态（移动端默认收起，桌面端默认展开）
  const [filterPanelOpen, setFilterPanelOpen] = useState(() => {
    if (typeof window !== 'undefined') {
      return window.innerWidth >= 640; // sm 断点
    }
    return true;
  });
  
  // 批量测试相关状态
  const [batchTesting, setBatchTesting] = useState(false);
  const [batchTestProgress, setBatchTestProgress] = useState<BatchTestProgress>({
    total: 0,
    completed: 0,
    success: 0,
    failed: 0,
    testing: 0
  });
  const [testAbortController, setTestAbortController] = useState<AbortController | null>(null);
  const [associationTestResults, setAssociationTestResults] = useState<Record<number, AssociationBatchTestResult>>({});

  const [blacklistDialogOpen, setBlacklistDialogOpen] = useState(false);
  const [blacklistedIds, setBlacklistedIds] = useState<number[]>([]);
  const [blacklistLoading, setBlacklistLoading] = useState(false);
  const [blacklistSaving, setBlacklistSaving] = useState(false);
  const [blacklistSearchTerm, setBlacklistSearchTerm] = useState("");
  const [blacklistFilter, setBlacklistFilter] = useState<BlacklistFilter>("all");

  // 拉黑管理过滤逻辑
  const filteredProviders = useMemo(() => {
    let result = providers;
    
    // 应用拉黑状态筛选
    if (blacklistFilter === 'blacklisted') {
      result = result.filter(provider => blacklistedIds.includes(provider.ID));
    } else if (blacklistFilter === 'not-blacklisted') {
      result = result.filter(provider => !blacklistedIds.includes(provider.ID));
    }
    
    // 应用搜索过滤
    if (blacklistSearchTerm.trim()) {
      const term = blacklistSearchTerm.toLowerCase().trim();
      result = result.filter(provider => 
        provider.Name.toLowerCase().includes(term) || 
        provider.Type.toLowerCase().includes(term)
      );
    }
    
    return result;
  }, [providers, blacklistedIds, blacklistFilter, blacklistSearchTerm]);

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
      tool_call: true,
      structured_output: false,
      image: false,
      with_header: false,
      weight: 5,
      priority: 10,
      customer_headers: [],
    },
  });
  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: "customer_headers",
  });

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
  }, [models, searchParams, form, setSearchParams, selectedModelId]);

  useEffect(() => {
    if (!templateEditorOpen || !selectedModelId) return;
    setTemplateLoading(true);
    getModelTemplate(selectedModelId)
      .then((data) => setTemplateData(data))
      .catch((err) => {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`加载模板失败: ${message}`);
      })
      .finally(() => setTemplateLoading(false));
  }, [templateEditorOpen, selectedModelId]);

  useEffect(() => {
    if (!blacklistDialogOpen) return;
    setBlacklistLoading(true);
    getProviderBlacklist()
      .then((data) => setBlacklistedIds(data.blacklisted_ids))
      .catch((err) => toast.error(`加载黑名单失败: ${err instanceof Error ? err.message : String(err)}`))
      .finally(() => setBlacklistLoading(false));
  }, [blacklistDialogOpen]);

  const handleSaveBlacklist = async () => {
    setBlacklistSaving(true);
    try {
      await updateProviderBlacklist(blacklistedIds);
      toast.success("黑名单已保存");
      setBlacklistDialogOpen(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`保存黑名单失败: ${message}`);
    } finally {
      setBlacklistSaving(false);
    }
  };

  const handleToggleBlacklist = (providerId: number, checked: boolean) => {
    setBlacklistedIds((prev) =>
      checked ? [...prev, providerId] : prev.filter((id) => id !== providerId)
    );
  };

  const buildPayload = buildAssociationPayload;

  const fetchModels = useCallback(async () => {
    try {
      const data = await getModels();
      setModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型列表失败: ${message}`);
      console.error(err);
    }
  }, []);

  const rebuildProviderModels = useCallback((providerList: Provider[]) => {
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
  }, []);

  const fetchProviders = useCallback(async () => {
    try {
      const data = await getProviders();
      setProviders(data);
      rebuildProviderModels(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商列表失败: ${message}`);
      console.error(err);
    }
  }, [rebuildProviderModels]);

  const fetchSettings = useCallback(async () => {
    try {
      const data = await getSettings();
      setSettings(data);
    } catch (err) {
      console.error("获取系统设置失败", err);
    }
  }, []);

  const loadProviderStatus = useCallback(async (providers: ModelWithProvider[], modelId: number) => {
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
  }, [models]);

  const fetchModelProviders = useCallback(async (modelId: number) => {
    try {
      setLoading(true);
      const data = await getModelProviders(modelId);
      setModelProviders(data.map(item => ({
        ...item,
        CustomerHeaders: item.CustomerHeaders || {}
      })));
      // 异步加载状态数据
      void loadProviderStatus(data, modelId);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型提供商关联列表失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [loadProviderStatus]);

  useEffect(() => {
    Promise.all([fetchModels(), fetchProviders(), fetchSettings()]).finally(() => {
      setLoading(false);
    });
  }, [fetchModels, fetchProviders, fetchSettings]);

  useEffect(() => {
    if (selectedModelId) {
      void fetchModelProviders(selectedModelId);
    }
  }, [selectedModelId, fetchModelProviders]);

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
        tool_call: true,
        structured_output: false,
        image: false,
        with_header: false,
        weight: settings?.auto_weight_decay_default || 5,
        priority: settings?.auto_priority_decay_default || 10,
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

  const handleConnectivityTest = async (id: number): Promise<ModelProviderTestResult> => {
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

  const handleStructuredOutputTest = async (id: number): Promise<ModelProviderTestResult> => {
    try {
      setStructuredTestResults(prev => ({
        ...prev,
        [id]: { loading: true, result: null }
      }));

      const result = await testModelProviderStructuredOutput(id);
      setStructuredTestResults(prev => ({
        ...prev,
        [id]: { loading: false, result }
      }));
      return result;
    } catch (err) {
      setStructuredTestResults(prev => ({
        ...prev,
        [id]: { loading: false, result: { passed: false, error: "测试失败" + err } }
      }));
      console.error(err);
      return { passed: false, error: "测试失败" + err };
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
    } else if (testType === "react") {
      await handleReactTest(selectedTestId);
    } else {
      await handleStructuredOutputTest(selectedTestId);
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
    // 始终使用设置中的默认权重和优先级值
    const defaultWeight = settings?.auto_weight_decay_default || 5;
    const defaultPriority = settings?.auto_priority_decay_default || 10;
    form.reset({
      model_id: selectedModelId || 0,
      provider_name: "",
      provider_id: 0,
      tool_call: true,
      structured_output: false,
      image: false,
      with_header: false,
      weight: defaultWeight,
      priority: defaultPriority,
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

  const handleBatchUpdateStatus = async (status: boolean) => {
    if (selectedAssociationIds.length === 0) {
      toast.error("请先选择要操作的关联");
      return;
    }
    
    setBatchUpdatingStatus(true);
    try {
      const result = await batchUpdateModelProvidersStatus(selectedAssociationIds, status);
      toast.success(`成功${status ? '启用' : '停用'} ${result.updated} 个关联`);
      
      // 更新本地状态
      setModelProviders(prev =>
        prev.map(item =>
          selectedAssociationIds.includes(item.ID)
            ? { ...item, Status: status }
            : item
        )
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`批量${status ? '启用' : '停用'}失败: ${message}`);
    } finally {
      setBatchUpdatingStatus(false);
    }
  };

  const handleAutoAssociate = async () => {
    try {
      setPreviewType("associate");
      const data = await previewAutoAssociate();
      setPreviewData(data);
      setPreviewDialogOpen(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取预览失败: ${message}`);
    }
  };

  const handleCleanInvalid = async () => {
    try {
      setPreviewType("clean");
      const data = await previewCleanInvalid();
      setPreviewData(data);
      setPreviewDialogOpen(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取预览失败: ${message}`);
    }
  };

  const handleResetWeights = async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setResettingWeights(true);
      const result = await resetModelWeights(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(
        result.updated > 0
          ? `已重置 ${result.updated} 个模型关联的权重到 ${result.default_weight}`
          : `所有模型关联已处于默认权重 ${result.default_weight}`
      );
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`重置权重失败: ${message}`);
    } finally {
      setResettingWeights(false);
    }
  };

  const handleResetPriorities = async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setResettingPriorities(true);
      const result = await resetModelPriorities(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(
        result.updated > 0
          ? `已重置 ${result.updated} 个模型关联的优先级到 ${result.default_priority}`
          : `所有模型关联已处于默认优先级 ${result.default_priority}`
      );
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`重置优先级失败: ${message}`);
    } finally {
      setResettingPriorities(false);
    }
  };

  const handleEnableAssociations = async () => {
    if (!selectedModelId && !isGlobalScope) return;
    try {
      setEnablingAssociations(true);
      const result = await enableAllAssociations(isGlobalScope ? undefined : (selectedModelId ?? undefined));
      toast.success(result.updated > 0 ? `已启用 ${result.updated} 个模型关联` : "所有模型关联已处于启用状态");
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`启用关联失败: ${message}`);
    } finally {
      setEnablingAssociations(false);
    }
  };

  const executePreviewAction = async () => {
    try {
      setExecuting(true);
      if (previewType === "associate") {
        const result = await autoAssociateModels();
        toast.success(`成功添加 ${result.added} 个关联`);
      } else {
        const result = await cleanInvalidAssociations();
        toast.success(`成功清除 ${result.removed} 个无效关联`);
      }
      setPreviewDialogOpen(false);
      if (selectedModelId) {
        fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    } finally {
      setExecuting(false);
    }
  };

  const handleToggleTemplateEditor = () => {
    setTemplateEditorOpen((prev) => !prev);
  };

  const handleAddTemplateItem = async () => {
    if (!selectedModelId) return;
    const name = templateNewItem.trim();
    if (!name) return;
    setTemplateLoading(true);
    try {
      const data = await addModelTemplateItem(selectedModelId, name);
      setTemplateData(data);
      setTemplateNewItem("");
      toast.success("已添加模板项");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加模板项失败: ${message}`);
    } finally {
      setTemplateLoading(false);
    }
  };

  const handleDeleteTemplateItem = async (name: string) => {
    if (!selectedModelId) return;
    setTemplateLoading(true);
    try {
      const data = await deleteModelTemplateItem(selectedModelId, name);
      setTemplateData(data);
      toast.success("已删除手动模板项");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除模板项失败: ${message}`);
    } finally {
      setTemplateLoading(false);
    }
  };

  const handleModelChange = (modelId: string) => {
    const id = parseInt(modelId);
    setSelectedModelId(id);
    setSelectedAssociationIds([]); // 切换模型时清空选择
    setSelectedProviderModels([]);
    setTemplateEditorOpen(false);
    setAssociationTestResults({}); // 切换模型时清空测试结果
    setSelectedStatusFilter("all"); // 切换模型时重置启用状态筛选器
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set("modelId", id.toString());
    setSearchParams(nextParams);
    form.setValue("model_id", id);
  };

  // 批量测试核心逻辑
  const testSingleAssociationInBatch = async (
    associationId: number,
    testing: Set<number>,
    signal: AbortSignal,
    counters: { success: number; failed: number }
  ) => {
    if (signal.aborted) {
      testing.delete(associationId);
      return;
    }

    try {
      setAssociationTestResults(prev => ({
        ...prev,
        [associationId]: { loading: true, success: null }
      }));

      await testModelProvider(associationId);
      
      setAssociationTestResults(prev => ({
        ...prev,
        [associationId]: { loading: false, success: true }
      }));
      
      // 使用局部计数器
      counters.success++;
      
      setBatchTestProgress(prev => ({
        ...prev,
        completed: prev.completed + 1,
        success: counters.success,
        testing: testing.size - 1
      }));
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setAssociationTestResults(prev => ({
        ...prev,
        [associationId]: { loading: false, success: false, error: message }
      }));
      
      // 使用局部计数器
      counters.failed++;
      
      setBatchTestProgress(prev => ({
        ...prev,
        completed: prev.completed + 1,
        failed: counters.failed,
        testing: testing.size - 1
      }));
    } finally {
      testing.delete(associationId);
    }
  };

  const startBatchTest = async (associationIds: number[]) => {
    if (associationIds.length === 0) {
      toast.error("没有可测试的关联");
      return;
    }

    setBatchTesting(true);
    setBatchTestProgress({
      total: associationIds.length,
      completed: 0,
      success: 0,
      failed: 0,
      testing: 0
    });

    const abortController = new AbortController();
    setTestAbortController(abortController);

    // 添加局部计数器
    const counters = {
      success: 0,
      failed: 0
    };

    const concurrency = 3;
    const queue = [...associationIds];
    const testing = new Set<number>();
    const promises: Promise<void>[] = [];

    try {
      while (queue.length > 0 && !abortController.signal.aborted) {
        while (testing.size < concurrency && queue.length > 0) {
          const id = queue.shift()!;
          testing.add(id);
          
          setBatchTestProgress(prev => ({
            ...prev,
            testing: testing.size
          }));

          // 传递计数器引用
          const testPromise = testSingleAssociationInBatch(id, testing, abortController.signal, counters);
          promises.push(testPromise);
        }

        await new Promise(resolve => setTimeout(resolve, 100));
      }

      // 等待所有测试任务真正完成
      await Promise.all(promises);

      // 使用局部计数器显示结果
      if (!abortController.signal.aborted) {
        toast.success(
          `批量测试完成：成功 ${counters.success} 个，失败 ${counters.failed} 个`,
          { duration: 5000 }
        );
      }
    } finally {
      setBatchTesting(false);
      setTestAbortController(null);
    }
  };

  const handleBatchTestAll = async () => {
    const ids = filteredModelProviders.map(mp => mp.ID);
    await startBatchTest(ids);
  };

  const handleBatchTestSelected = async () => {
    if (selectedAssociationIds.length === 0) {
      toast.error("请先选择要测试的关联");
      return;
    }
    await startBatchTest(selectedAssociationIds);
  };

  const handleCancelBatchTest = () => {
    if (testAbortController) {
      testAbortController.abort();
      toast.info("已取消批量测试");
    }
  };

  const selectAllSuccessful = () => {
    const visibleIds = new Set(filteredModelProviders.map(mp => mp.ID));
    const successfulIds = Object.entries(associationTestResults)
      .filter(([id, result]) => result.success === true && visibleIds.has(parseInt(id)))
      .map(([id]) => parseInt(id));
    
    if (successfulIds.length === 0) {
      toast.info("当前列表中没有测试成功的项");
      return;
    }
    
    setSelectedAssociationIds(successfulIds);
    
    toast.success(`已选择 ${successfulIds.length} 个测试成功的项`);
  };

  const selectAllFailed = () => {
    const visibleIds = new Set(filteredModelProviders.map(mp => mp.ID));
    const failedIds = Object.entries(associationTestResults)
      .filter(([id, result]) => result.success === false && visibleIds.has(parseInt(id)))
      .map(([id]) => parseInt(id));
    
    if (failedIds.length === 0) {
      toast.info("当前列表中没有测试失败的项");
      return;
    }
    
    setSelectedAssociationIds(failedIds);
    
    toast.success(`已选择 ${failedIds.length} 个测试失败的项`);
  };

  // ✅ 新增：清除批量测试结果
  const clearBatchTestResults = () => {
    setBatchTestProgress({
      total: 0,
      completed: 0,
      success: 0,
      failed: 0,
      testing: 0
    });
    setAssociationTestResults({});
    toast.info("已清除测试结果");
  };

  const toggleProviderCollapse = (providerId: number) => {
    setCollapsedProviders((prev) => ({
      ...prev,
      [providerId]: !prev[providerId],
    }));
  };

  // 获取唯一的提供商类型列表
  const providerTypes = Array.from(new Set(providers.map(p => p.Type).filter(Boolean)));

  // 根据选择的提供商类型、具体提供商、启用状态和搜索关键词过滤模型提供商关联
  const filteredModelProviders = modelProviders.filter(association => {
    const provider = providers.find(p => p.ID === association.ProviderID);

    // 提供商类型筛选
    const typeMatch = selectedProviderType === "all" || provider?.Type === selectedProviderType;

    // 具体提供商筛选
    const providerMatch = selectedProviderFilter === "all" || association.ProviderID.toString() === selectedProviderFilter;

    // 启用状态筛选
    const statusMatch =
      selectedStatusFilter === "all" ||
      (selectedStatusFilter === "enabled" && (association.Status ?? true)) ||
      (selectedStatusFilter === "disabled" && !(association.Status ?? true));

    // 搜索关键词筛选
    const keyword = searchKeyword.toLowerCase().trim();
    const searchMatch = !keyword ||
      association.ProviderModel.toLowerCase().includes(keyword) ||
      (provider?.Name ?? "").toLowerCase().includes(keyword) ||
      (provider?.Type ?? "").toLowerCase().includes(keyword) ||
      association.ID.toString().includes(keyword);

    return typeMatch && providerMatch && statusMatch && searchMatch;
  });

  const hasAssociationFilter = selectedProviderType !== "all" || selectedProviderFilter !== "all" || selectedStatusFilter !== "all" || searchKeyword.trim() !== "";

  // 计算激活的筛选条件数量
  const activeFilterCount = [
    selectedProviderType !== 'all',
    selectedProviderFilter !== 'all',
    selectedStatusFilter !== 'all',
    searchKeyword.trim() !== ''
  ].filter(Boolean).length;

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
  const selectedModel = models.find((model) => model.ID === selectedModelId) || null;
  const isGlobalScope = operationScope === "all";

  if (loading && models.length === 0 && providers.length === 0) return <Loading message="加载模型和提供商" />;
  return (
    <div className="h-full min-h-0 flex flex-col gap-3 p-1">
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-xl font-semibold tracking-tight">模型提供商关联</h2>
          <OperationScopeToolbar
            operationScope={operationScope}
            onOperationScopeChange={setOperationScope}
            selectedModelName={isGlobalScope ? "全部" : (selectedModel?.Name ?? "未选择")}
            selectedModelId={selectedModelId}
            resettingWeights={resettingWeights}
            resettingPriorities={resettingPriorities}
            enablingAssociations={enablingAssociations}
            onResetWeights={handleResetWeights}
            onResetPriorities={handleResetPriorities}
            onEnableAssociations={handleEnableAssociations}
          />
        </div>
      </div>

      <AssociationFilterPanel
        filterPanelOpen={filterPanelOpen}
        onFilterPanelOpenChange={setFilterPanelOpen}
        activeFilterCount={activeFilterCount}
        selectedModelId={selectedModelId}
        models={models}
        onModelChange={handleModelChange}
        selectedProviderType={selectedProviderType}
        onSelectedProviderTypeChange={setSelectedProviderType}
        selectedProviderFilter={selectedProviderFilter}
        onSelectedProviderFilterChange={setSelectedProviderFilter}
        selectedStatusFilter={selectedStatusFilter}
        onSelectedStatusFilterChange={setSelectedStatusFilter}
        searchKeyword={searchKeyword}
        onSearchKeywordChange={setSearchKeyword}
        providers={providers}
        providerTypes={providerTypes}
        selectedAssociationCount={selectedAssociationIds.length}
        batchUpdatingStatus={batchUpdatingStatus}
        batchTesting={batchTesting}
        filteredAssociationCount={filteredModelProviders.length}
        associationTestResults={associationTestResults}
        batchDeleteDialogOpen={batchDeleteDialogOpen}
        onBatchDeleteDialogOpenChange={setBatchDeleteDialogOpen}
        batchDeleting={batchDeleting}
        onBatchDeleteConfirm={handleBatchDeleteAssociations}
        onBatchUpdateStatus={handleBatchUpdateStatus}
        onBatchTestSelected={handleBatchTestSelected}
        onBatchTestAll={handleBatchTestAll}
        onSelectAllSuccessful={selectAllSuccessful}
        onSelectAllFailed={selectAllFailed}
        onToggleTemplateEditor={handleToggleTemplateEditor}
        onOpenBlacklistDialog={() => setBlacklistDialogOpen(true)}
        onAutoAssociate={handleAutoAssociate}
        onCleanInvalid={handleCleanInvalid}
        onOpenCreateDialog={openCreateDialog}
      />

      <BatchTestProgressCard
        batchTesting={batchTesting}
        batchTestProgress={batchTestProgress}
        associationTestResults={associationTestResults}
        onCancel={handleCancelBatchTest}
        onClear={clearBatchTestResults}
        onSelectSuccess={selectAllSuccessful}
        onSelectFailed={selectAllFailed}
      />

      {statusError && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {statusError}
        </div>
      )}

      <AssociationListSection
        loading={loading}
        selectedModelId={selectedModelId}
        hasAssociationFilter={hasAssociationFilter}
        associations={filteredModelProviders}
        providers={providers}
        selectedAssociationIds={selectedAssociationIds}
        isAllSelected={isAllAssociationsSelected}
        isPartialSelected={isPartialAssociationsSelected}
        providerStatus={providerStatus}
        healthStatus={healthStatus}
        statusUpdating={statusUpdating}
        associationTestResults={associationTestResults}
        deleteId={deleteId}
        onSelectAll={handleSelectAllAssociations}
        onSelectOne={handleSelectOneAssociation}
        onRefreshStatus={() => {
          if (selectedModelId) {
            void loadProviderStatus(modelProviders, selectedModelId);
          }
        }}
        onToggleStatus={handleStatusToggle}
        onEdit={openEditDialog}
        onOpenDelete={openDeleteDialog}
        onDeleteDialogChange={(openValue) => {
          if (!openValue) {
            setDeleteId(null);
          }
        }}
        onDeleteConfirm={handleDelete}
        onTest={handleTest}
      />

      <BlacklistDialog
        open={blacklistDialogOpen}
        onOpenChange={setBlacklistDialogOpen}
        providers={providers}
        filteredProviders={filteredProviders}
        blacklistedIds={blacklistedIds}
        loading={blacklistLoading}
        saving={blacklistSaving}
        searchTerm={blacklistSearchTerm}
        filter={blacklistFilter}
        onSearchTermChange={setBlacklistSearchTerm}
        onFilterChange={setBlacklistFilter}
        onToggle={handleToggleBlacklist}
        onSave={handleSaveBlacklist}
        onCancel={() => {
          setBlacklistDialogOpen(false);
          setBlacklistSearchTerm("");
          setBlacklistFilter("all");
        }}
      />

      <TemplateEditorDialog
        open={templateEditorOpen}
        onOpenChange={setTemplateEditorOpen}
        selectedModelId={selectedModelId}
        loading={templateLoading}
        templateData={templateData}
        newItem={templateNewItem}
        onNewItemChange={setTemplateNewItem}
        onAdd={() => {
          void handleAddTemplateItem();
        }}
        onDelete={(name) => {
          void handleDeleteTemplateItem(name);
        }}
      />

      <AssociationFormDialog
        open={open}
        onOpenChange={setOpen}
        editingAssociation={editingAssociation}
        form={form}
        models={models}
        providers={providers}
        selectedProviderModels={selectedProviderModels}
        isSubmitting={isSubmitting}
        headerFields={headerFields}
        appendHeader={appendHeader}
        removeHeader={removeHeader}
        onSubmitCreate={handleCreate}
        onSubmitUpdate={handleUpdate}
        onOpenModelListDialog={() => {
          setModelSearchKeyword("");
          setModelListDialogOpen(true);
        }}
        onClearSelectedProviderModels={() => setSelectedProviderModels([])}
        onRemoveSelectedProviderModel={(selectionKey) => {
          setSelectedProviderModels((prev) =>
            prev.filter((item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey)
          );
        }}
        onProviderChange={() => setSelectedProviderModels([])}
      />

      <TestDialog
        open={testDialogOpen}
        onOpenChange={setTestDialogOpen}
        testType={testType}
        onTestTypeChange={setTestType}
        selectedTestId={selectedTestId}
        testResults={testResults}
        structuredTestResults={structuredTestResults}
        reactTestResult={reactTestResult}
        onClose={dialogClose}
        onExecute={() => {
          void executeTest();
        }}
      />

      <ModelListDialog
        open={modelListDialogOpen}
        onOpenChange={setModelListDialogOpen}
        modelSearchKeyword={modelSearchKeyword}
        onModelSearchKeywordChange={setModelSearchKeyword}
        loadingProviderModels={loadingProviderModels}
        providerModels={providerModels}
        visibleProviderGroups={visibleProviderGroups}
        visibleAvailableModels={visibleAvailableModels}
        visibleExistingCount={visibleExistingCount}
        selectedProviderModels={selectedProviderModels}
        selectedKeys={selectedKeys}
        existingAssociationKeys={existingAssociationKeys}
        collapsedProviders={collapsedProviders}
        onToggleProviderCollapse={toggleProviderCollapse}
        onSelectAllVisibleAvailable={() => {
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
        onClearSelection={() => setSelectedProviderModels([])}
        onToggleModelSelection={(model, checkedValue, selectionKey) => {
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
            return;
          }

          setSelectedProviderModels((prev) =>
            prev.filter((item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey)
          );
        }}
      />

      <PreviewDialog
        open={previewDialogOpen}
        onOpenChange={setPreviewDialogOpen}
        type={previewType}
        data={previewData}
        executing={executing}
        onConfirm={() => {
          void executePreviewAction();
        }}
      />
    </div>
  );
}

