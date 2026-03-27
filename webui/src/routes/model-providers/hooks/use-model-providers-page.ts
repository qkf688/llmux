import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { getAuthToken } from "@/stores/auth";
import {
  selectModelProvidersAssociationTestResults,
  selectModelProvidersBatchDeleteDialogOpen,
  selectModelProvidersBatchDeleting,
  selectModelProvidersBatchTestProgress,
  selectModelProvidersBatchTesting,
  selectModelProvidersBatchUpdatingStatus,
  selectModelProvidersBlacklistDialogOpen,
  selectModelProvidersBlacklistFilter,
  selectModelProvidersBlacklistLoading,
  selectModelProvidersBlacklistSaving,
  selectModelProvidersBlacklistSearchTerm,
  selectModelProvidersBlacklistedIds,
  selectModelProvidersCollapsedProviders,
  selectModelProvidersDeleteId,
  selectModelProvidersEditingAssociation,
  selectModelProvidersEnablingAssociations,
  selectModelProvidersExecuting,
  selectModelProvidersFilterPanelOpen,
  selectModelProvidersIsSubmitting,
  selectModelProvidersLoading,
  selectModelProvidersLoadingProviderModels,
  selectModelProvidersModelListDialogOpen,
  selectModelProvidersModelSearchKeyword,
  selectModelProvidersOpen,
  selectModelProvidersOperationScope,
  selectModelProvidersPreviewDialogOpen,
  selectModelProvidersPreviewType,
  selectModelProvidersReactTestResult,
  selectModelProvidersResettingPriorities,
  selectModelProvidersResettingWeights,
  selectModelProvidersSearchKeyword,
  selectModelProvidersSelectedAssociationIds,
  selectModelProvidersSelectedProviderFilter,
  selectModelProvidersSelectedProviderModels,
  selectModelProvidersSelectedProviderType,
  selectModelProvidersSelectedStatusFilter,
  selectModelProvidersSelectedTestId,
  selectModelProvidersTemplateEditorOpen,
  selectModelProvidersTemplateLoading,
  selectModelProvidersTemplateNewItem,
  selectModelProvidersTestDialogOpen,
  selectModelProvidersTestType,
  selectResetModelProvidersTransient,
  selectSetModelProvidersAssociationTestResults,
  selectSetModelProvidersBatchDeleteDialogOpen,
  selectSetModelProvidersBatchDeleting,
  selectSetModelProvidersBatchTestProgress,
  selectSetModelProvidersBatchTesting,
  selectSetModelProvidersBatchUpdatingStatus,
  selectSetModelProvidersBlacklistDialogOpen,
  selectSetModelProvidersBlacklistFilter,
  selectSetModelProvidersBlacklistLoading,
  selectSetModelProvidersBlacklistSaving,
  selectSetModelProvidersBlacklistSearchTerm,
  selectSetModelProvidersBlacklistedIds,
  selectSetModelProvidersCollapsedProviders,
  selectSetModelProvidersDeleteId,
  selectSetModelProvidersEditingAssociation,
  selectSetModelProvidersEnablingAssociations,
  selectSetModelProvidersExecuting,
  selectSetModelProvidersFilterPanelOpen,
  selectSetModelProvidersIsSubmitting,
  selectSetModelProvidersLoading,
  selectSetModelProvidersLoadingProviderModels,
  selectSetModelProvidersModelListDialogOpen,
  selectSetModelProvidersModelSearchKeyword,
  selectSetModelProvidersOpen,
  selectSetModelProvidersOperationScope,
  selectSetModelProvidersPreviewDialogOpen,
  selectSetModelProvidersPreviewType,
  selectSetModelProvidersReactTestResult,
  selectSetModelProvidersResettingPriorities,
  selectSetModelProvidersResettingWeights,
  selectSetModelProvidersSearchKeyword,
  selectSetModelProvidersSelectedAssociationIds,
  selectSetModelProvidersSelectedProviderFilter,
  selectSetModelProvidersSelectedProviderModels,
  selectSetModelProvidersSelectedProviderType,
  selectSetModelProvidersSelectedStatusFilter,
  selectSetModelProvidersSelectedTestId,
  selectSetModelProvidersTemplateEditorOpen,
  selectSetModelProvidersTemplateLoading,
  selectSetModelProvidersTemplateNewItem,
  selectSetModelProvidersTestDialogOpen,
  selectSetModelProvidersTestType,
  useModelProvidersPageStore,
} from "@/stores/model-providers";
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
import { formSchema, type FormValues } from "../form-schema";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";
import { buildAssociationPayload } from "../utils/payload";
import { buildSelectionKey } from "../utils/selection";

export function useModelProvidersPage() {
  const [modelProviders, setModelProviders] = useState<ModelWithProvider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const [providerStatus, setProviderStatus] = useState<Record<number, boolean[]>>({});
  const [healthStatus, setHealthStatus] = useState<Record<number, boolean[]>>({});

  const loading = useModelProvidersPageStore(selectModelProvidersLoading);
  const setLoading = useModelProvidersPageStore(selectSetModelProvidersLoading);
  const open = useModelProvidersPageStore(selectModelProvidersOpen);
  const setOpen = useModelProvidersPageStore(selectSetModelProvidersOpen);
  const editingAssociation = useModelProvidersPageStore(selectModelProvidersEditingAssociation);
  const setEditingAssociation = useModelProvidersPageStore(selectSetModelProvidersEditingAssociation);
  const deleteId = useModelProvidersPageStore(selectModelProvidersDeleteId);
  const setDeleteId = useModelProvidersPageStore(selectSetModelProvidersDeleteId);

  const testDialogOpen = useModelProvidersPageStore(selectModelProvidersTestDialogOpen);
  const setTestDialogOpen = useModelProvidersPageStore(selectSetModelProvidersTestDialogOpen);
  const selectedTestId = useModelProvidersPageStore(selectModelProvidersSelectedTestId);
  const setSelectedTestId = useModelProvidersPageStore(selectSetModelProvidersSelectedTestId);
  const testType = useModelProvidersPageStore(selectModelProvidersTestType);
  const setTestType = useModelProvidersPageStore(selectSetModelProvidersTestType);
  const reactTestResult = useModelProvidersPageStore(selectModelProvidersReactTestResult);
  const setReactTestResult = useModelProvidersPageStore(selectSetModelProvidersReactTestResult);
  const isSubmitting = useModelProvidersPageStore(selectModelProvidersIsSubmitting);
  const setIsSubmitting = useModelProvidersPageStore(selectSetModelProvidersIsSubmitting);

  const loadingProviderModels = useModelProvidersPageStore(selectModelProvidersLoadingProviderModels);
  const setLoadingProviderModels = useModelProvidersPageStore(selectSetModelProvidersLoadingProviderModels);
  const modelListDialogOpen = useModelProvidersPageStore(selectModelProvidersModelListDialogOpen);
  const setModelListDialogOpen = useModelProvidersPageStore(selectSetModelProvidersModelListDialogOpen);
  const modelSearchKeyword = useModelProvidersPageStore(selectModelProvidersModelSearchKeyword);
  const setModelSearchKeyword = useModelProvidersPageStore(selectSetModelProvidersModelSearchKeyword);
  const selectedProviderModels = useModelProvidersPageStore(selectModelProvidersSelectedProviderModels);
  const setSelectedProviderModels = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderModels);
  const selectedAssociationIds = useModelProvidersPageStore(selectModelProvidersSelectedAssociationIds);
  const setSelectedAssociationIds = useModelProvidersPageStore(selectSetModelProvidersSelectedAssociationIds);
  const collapsedProviders = useModelProvidersPageStore(selectModelProvidersCollapsedProviders);
  const setCollapsedProviders = useModelProvidersPageStore(selectSetModelProvidersCollapsedProviders);

  const batchDeleteDialogOpen = useModelProvidersPageStore(selectModelProvidersBatchDeleteDialogOpen);
  const setBatchDeleteDialogOpen = useModelProvidersPageStore(selectSetModelProvidersBatchDeleteDialogOpen);
  const batchDeleting = useModelProvidersPageStore(selectModelProvidersBatchDeleting);
  const setBatchDeleting = useModelProvidersPageStore(selectSetModelProvidersBatchDeleting);
  const batchUpdatingStatus = useModelProvidersPageStore(selectModelProvidersBatchUpdatingStatus);
  const setBatchUpdatingStatus = useModelProvidersPageStore(selectSetModelProvidersBatchUpdatingStatus);

  const searchKeyword = useModelProvidersPageStore(selectModelProvidersSearchKeyword);
  const setSearchKeyword = useModelProvidersPageStore(selectSetModelProvidersSearchKeyword);
  const selectedProviderType = useModelProvidersPageStore(selectModelProvidersSelectedProviderType);
  const setSelectedProviderType = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderType);
  const selectedProviderFilter = useModelProvidersPageStore(selectModelProvidersSelectedProviderFilter);
  const setSelectedProviderFilter = useModelProvidersPageStore(selectSetModelProvidersSelectedProviderFilter);
  const selectedStatusFilter = useModelProvidersPageStore(selectModelProvidersSelectedStatusFilter);
  const setSelectedStatusFilter = useModelProvidersPageStore(selectSetModelProvidersSelectedStatusFilter);
  const operationScope = useModelProvidersPageStore(selectModelProvidersOperationScope);
  const setOperationScope = useModelProvidersPageStore(selectSetModelProvidersOperationScope);
  const filterPanelOpen = useModelProvidersPageStore(selectModelProvidersFilterPanelOpen);
  const setFilterPanelOpen = useModelProvidersPageStore(selectSetModelProvidersFilterPanelOpen);

  const previewDialogOpen = useModelProvidersPageStore(selectModelProvidersPreviewDialogOpen);
  const setPreviewDialogOpen = useModelProvidersPageStore(selectSetModelProvidersPreviewDialogOpen);
  const previewType = useModelProvidersPageStore(selectModelProvidersPreviewType);
  const setPreviewType = useModelProvidersPageStore(selectSetModelProvidersPreviewType);
  const executing = useModelProvidersPageStore(selectModelProvidersExecuting);
  const setExecuting = useModelProvidersPageStore(selectSetModelProvidersExecuting);

  const templateEditorOpen = useModelProvidersPageStore(selectModelProvidersTemplateEditorOpen);
  const setTemplateEditorOpen = useModelProvidersPageStore(selectSetModelProvidersTemplateEditorOpen);
  const templateLoading = useModelProvidersPageStore(selectModelProvidersTemplateLoading);
  const setTemplateLoading = useModelProvidersPageStore(selectSetModelProvidersTemplateLoading);
  const templateNewItem = useModelProvidersPageStore(selectModelProvidersTemplateNewItem);
  const setTemplateNewItem = useModelProvidersPageStore(selectSetModelProvidersTemplateNewItem);

  const resettingWeights = useModelProvidersPageStore(selectModelProvidersResettingWeights);
  const setResettingWeights = useModelProvidersPageStore(selectSetModelProvidersResettingWeights);
  const resettingPriorities = useModelProvidersPageStore(selectModelProvidersResettingPriorities);
  const setResettingPriorities = useModelProvidersPageStore(selectSetModelProvidersResettingPriorities);
  const enablingAssociations = useModelProvidersPageStore(selectModelProvidersEnablingAssociations);
  const setEnablingAssociations = useModelProvidersPageStore(selectSetModelProvidersEnablingAssociations);

  const batchTesting = useModelProvidersPageStore(selectModelProvidersBatchTesting);
  const setBatchTesting = useModelProvidersPageStore(selectSetModelProvidersBatchTesting);
  const batchTestProgress = useModelProvidersPageStore(selectModelProvidersBatchTestProgress);
  const setBatchTestProgress = useModelProvidersPageStore(selectSetModelProvidersBatchTestProgress);
  const associationTestResults = useModelProvidersPageStore(selectModelProvidersAssociationTestResults);
  const setAssociationTestResults = useModelProvidersPageStore(selectSetModelProvidersAssociationTestResults);

  const blacklistDialogOpen = useModelProvidersPageStore(selectModelProvidersBlacklistDialogOpen);
  const setBlacklistDialogOpen = useModelProvidersPageStore(selectSetModelProvidersBlacklistDialogOpen);
  const blacklistedIds = useModelProvidersPageStore(selectModelProvidersBlacklistedIds);
  const setBlacklistedIds = useModelProvidersPageStore(selectSetModelProvidersBlacklistedIds);
  const blacklistLoading = useModelProvidersPageStore(selectModelProvidersBlacklistLoading);
  const setBlacklistLoading = useModelProvidersPageStore(selectSetModelProvidersBlacklistLoading);
  const blacklistSaving = useModelProvidersPageStore(selectModelProvidersBlacklistSaving);
  const setBlacklistSaving = useModelProvidersPageStore(selectSetModelProvidersBlacklistSaving);
  const blacklistSearchTerm = useModelProvidersPageStore(selectModelProvidersBlacklistSearchTerm);
  const setBlacklistSearchTerm = useModelProvidersPageStore(selectSetModelProvidersBlacklistSearchTerm);
  const blacklistFilter = useModelProvidersPageStore(selectModelProvidersBlacklistFilter);
  const setBlacklistFilter = useModelProvidersPageStore(selectSetModelProvidersBlacklistFilter);

  const resetTransient = useModelProvidersPageStore(selectResetModelProvidersTransient);

  const [selectedModelId, setSelectedModelId] = useState<number | null>(null);
  const [testResults, setTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [structuredTestResults, setStructuredTestResults] = useState<
    Record<number, { loading: boolean; result: ModelProviderTestResult | null }>
  >({});
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [previewData, setPreviewData] = useState<AssociationPreview[]>([]);
  const [templateData, setTemplateData] = useState<ModelTemplate | null>(null);
  const [testAbortController, setTestAbortController] = useState<AbortController | null>(null);

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
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

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
  }, [templateEditorOpen, selectedModelId, setTemplateLoading]);

  useEffect(() => {
    if (!blacklistDialogOpen) return;
    setBlacklistLoading(true);
    getProviderBlacklist()
      .then((data) => setBlacklistedIds(data.blacklisted_ids))
      .catch((err) => toast.error(`加载黑名单失败: ${err instanceof Error ? err.message : String(err)}`))
      .finally(() => setBlacklistLoading(false));
  }, [blacklistDialogOpen, setBlacklistLoading, setBlacklistedIds]);

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
  }, [setCollapsedProviders, setLoadingProviderModels]);

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
  }, [loadProviderStatus, setLoading]);

  useEffect(() => {
    Promise.all([fetchModels(), fetchProviders(), fetchSettings()]).finally(() => {
      setLoading(false);
    });
  }, [fetchModels, fetchProviders, fetchSettings, setLoading]);

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
      const token = getAuthToken();
      if (!token) {
        window.location.href = "/login";
        return;
      }
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
    setTemplateEditorOpen(!templateEditorOpen);
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

  const shouldShowInitialLoading = loading && models.length === 0 && providers.length === 0;

  const openBlacklistDialog = () => {
    setBlacklistDialogOpen(true);
  };

  const refreshStatus = () => {
    if (selectedModelId) {
      void loadProviderStatus(modelProviders, selectedModelId);
    }
  };

  const handleDeleteDialogChange = (openValue: boolean) => {
    if (!openValue) {
      setDeleteId(null);
    }
  };

  const cancelBlacklistDialog = () => {
    setBlacklistDialogOpen(false);
    setBlacklistSearchTerm("");
    setBlacklistFilter("all");
  };

  const openModelListDialog = () => {
    setModelSearchKeyword("");
    setModelListDialogOpen(true);
  };

  const clearSelectedProviderModels = () => {
    setSelectedProviderModels([]);
  };

  const removeSelectedProviderModel = (selectionKey: string) => {
    setSelectedProviderModels((prev) =>
      prev.filter((item) => buildSelectionKey(item.providerId, item.modelId) !== selectionKey)
    );
  };

  const handleProviderChange = () => {
    setSelectedProviderModels([]);
  };

  const executeTestNow = () => {
    void executeTest();
  };

  const selectAllVisibleAvailable = () => {
    setSelectedProviderModels((prev) => {
      const merged = new Map(prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item]));
      visibleAvailableModels.forEach((model) => {
        merged.set(buildSelectionKey(model.providerId, model.id), {
          providerId: model.providerId,
          providerName: model.providerName,
          modelId: model.id,
        });
      });
      return Array.from(merged.values());
    });
  };

  const clearModelListSelection = () => {
    setSelectedProviderModels([]);
  };

  const toggleModelSelection = (model: ProviderModelWithOwner, checkedValue: boolean, selectionKey: string) => {
    if (checkedValue) {
      setSelectedProviderModels((prev) => {
        const merged = new Map(prev.map((item) => [buildSelectionKey(item.providerId, item.modelId), item]));
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
  };

  const confirmPreviewAction = () => {
    void executePreviewAction();
  };

  const addTemplateItem = () => {
    void handleAddTemplateItem();
  };

  const deleteTemplateItem = (name: string) => {
    void handleDeleteTemplateItem(name);
  };

  const operationScopeToolbarProps = {
    operationScope,
    onOperationScopeChange: setOperationScope,
    selectedModelName: isGlobalScope ? "全部" : (selectedModel?.Name ?? "未选择"),
    selectedModelId,
    resettingWeights,
    resettingPriorities,
    enablingAssociations,
    onResetWeights: handleResetWeights,
    onResetPriorities: handleResetPriorities,
    onEnableAssociations: handleEnableAssociations,
  };

  const associationFilterPanelProps = {
    filterPanelOpen,
    onFilterPanelOpenChange: setFilterPanelOpen,
    activeFilterCount,
    selectedModelId,
    models,
    onModelChange: handleModelChange,
    selectedProviderType,
    onSelectedProviderTypeChange: setSelectedProviderType,
    selectedProviderFilter,
    onSelectedProviderFilterChange: setSelectedProviderFilter,
    selectedStatusFilter,
    onSelectedStatusFilterChange: setSelectedStatusFilter,
    searchKeyword,
    onSearchKeywordChange: setSearchKeyword,
    providers,
    providerTypes,
    selectedAssociationCount: selectedAssociationIds.length,
    batchUpdatingStatus,
    batchTesting,
    filteredAssociationCount: filteredModelProviders.length,
    associationTestResults,
    batchDeleteDialogOpen,
    onBatchDeleteDialogOpenChange: setBatchDeleteDialogOpen,
    batchDeleting,
    onBatchDeleteConfirm: handleBatchDeleteAssociations,
    onBatchUpdateStatus: handleBatchUpdateStatus,
    onBatchTestSelected: handleBatchTestSelected,
    onBatchTestAll: handleBatchTestAll,
    onSelectAllSuccessful: selectAllSuccessful,
    onSelectAllFailed: selectAllFailed,
    onToggleTemplateEditor: handleToggleTemplateEditor,
    onOpenBlacklistDialog: openBlacklistDialog,
    onAutoAssociate: handleAutoAssociate,
    onCleanInvalid: handleCleanInvalid,
    onOpenCreateDialog: openCreateDialog,
  };

  const batchTestProgressCardProps = {
    batchTesting,
    batchTestProgress,
    associationTestResults,
    onCancel: handleCancelBatchTest,
    onClear: clearBatchTestResults,
    onSelectSuccess: selectAllSuccessful,
    onSelectFailed: selectAllFailed,
  };

  const associationListSectionProps = {
    loading,
    selectedModelId,
    hasAssociationFilter,
    associations: filteredModelProviders,
    providers,
    selectedAssociationIds,
    isAllSelected: isAllAssociationsSelected,
    isPartialSelected: isPartialAssociationsSelected,
    providerStatus,
    healthStatus,
    statusUpdating,
    associationTestResults,
    deleteId,
    onSelectAll: handleSelectAllAssociations,
    onSelectOne: handleSelectOneAssociation,
    onRefreshStatus: refreshStatus,
    onToggleStatus: handleStatusToggle,
    onEdit: openEditDialog,
    onOpenDelete: openDeleteDialog,
    onDeleteDialogChange: handleDeleteDialogChange,
    onDeleteConfirm: handleDelete,
    onTest: handleTest,
  };

  const blacklistDialogProps = {
    open: blacklistDialogOpen,
    onOpenChange: setBlacklistDialogOpen,
    providers,
    filteredProviders,
    blacklistedIds,
    loading: blacklistLoading,
    saving: blacklistSaving,
    searchTerm: blacklistSearchTerm,
    filter: blacklistFilter,
    onSearchTermChange: setBlacklistSearchTerm,
    onFilterChange: setBlacklistFilter,
    onToggle: handleToggleBlacklist,
    onSave: handleSaveBlacklist,
    onCancel: cancelBlacklistDialog,
  };

  const templateEditorDialogProps = {
    open: templateEditorOpen,
    onOpenChange: setTemplateEditorOpen,
    selectedModelId,
    loading: templateLoading,
    templateData,
    newItem: templateNewItem,
    onNewItemChange: setTemplateNewItem,
    onAdd: addTemplateItem,
    onDelete: deleteTemplateItem,
  };

  const associationFormDialogProps = {
    open,
    onOpenChange: setOpen,
    editingAssociation,
    form,
    models,
    providers,
    selectedProviderModels,
    isSubmitting,
    headerFields,
    appendHeader,
    removeHeader,
    onSubmitCreate: handleCreate,
    onSubmitUpdate: handleUpdate,
    onOpenModelListDialog: openModelListDialog,
    onClearSelectedProviderModels: clearSelectedProviderModels,
    onRemoveSelectedProviderModel: removeSelectedProviderModel,
    onProviderChange: handleProviderChange,
  };

  const testDialogProps = {
    open: testDialogOpen,
    onOpenChange: setTestDialogOpen,
    testType,
    onTestTypeChange: setTestType,
    selectedTestId,
    testResults,
    structuredTestResults,
    reactTestResult,
    onClose: dialogClose,
    onExecute: executeTestNow,
  };

  const modelListDialogProps = {
    open: modelListDialogOpen,
    onOpenChange: setModelListDialogOpen,
    modelSearchKeyword,
    onModelSearchKeywordChange: setModelSearchKeyword,
    loadingProviderModels,
    providerModels,
    visibleProviderGroups,
    visibleAvailableModels,
    visibleExistingCount,
    selectedProviderModels,
    selectedKeys,
    existingAssociationKeys,
    collapsedProviders,
    onToggleProviderCollapse: toggleProviderCollapse,
    onSelectAllVisibleAvailable: selectAllVisibleAvailable,
    onClearSelection: clearModelListSelection,
    onToggleModelSelection: toggleModelSelection,
  };

  const previewDialogProps = {
    open: previewDialogOpen,
    onOpenChange: setPreviewDialogOpen,
    type: previewType,
    data: previewData,
    executing,
    onConfirm: confirmPreviewAction,
  };

  return {
    shouldShowInitialLoading,
    statusError,
    operationScopeToolbarProps,
    associationFilterPanelProps,
    batchTestProgressCardProps,
    associationListSectionProps,
    blacklistDialogProps,
    templateEditorDialogProps,
    associationFormDialogProps,
    testDialogProps,
    modelListDialogProps,
    previewDialogProps,
  };
}

