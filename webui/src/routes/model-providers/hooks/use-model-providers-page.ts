import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
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
  createModelProvider,
  deleteModelProvider,
  getModelProviderHealthStatus,
  getModelProviderStatus,
  getModelProviders,
  updateModelProvider,
  updateModelProviderStatus,
} from "@/lib/api";
import type {
  Model,
  ModelWithProvider,
  Provider,
  Settings,
} from "@/lib/api";
import { toast } from "sonner";
import { formSchema, type FormValues } from "../form-schema";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../types";
import { buildAssociationPayload } from "../utils/payload";
import { buildSelectionKey } from "../utils/selection";
import { useModelProvidersBatch } from "./use-model-providers-batch";
import { useModelProvidersBlacklist } from "./use-model-providers-blacklist";
import { useModelProvidersBootstrap } from "./use-model-providers-bootstrap";
import { useModelProvidersOperationScope } from "./use-model-providers-operation-scope";
import { useModelProvidersPreview } from "./use-model-providers-preview";
import { useModelProvidersTemplateEditor } from "./use-model-providers-template-editor";
import { useModelProvidersTesting } from "./use-model-providers-testing";

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
  const [statusUpdating, setStatusUpdating] = useState<Record<number, boolean>>({});
  const [statusError, setStatusError] = useState<string | null>(null);
  const [providerModelGroups, setProviderModelGroups] = useState<ProviderModelGroup[]>([]);
  const [providerModels, setProviderModels] = useState<ProviderModelWithOwner[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);

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

  useModelProvidersBootstrap({
    models,
    setModels,
    setProviders,
    setSettings,
    setLoading,
    setLoadingProviderModels,
    setProviderModelGroups,
    setProviderModels,
    setCollapsedProviders,
    resetTransient,
    selectedModelId,
    setSelectedModelId,
    searchParams,
    setSearchParams,
    setFormModelId: (value) => form.setValue("model_id", value),
  });

  const { filteredProviders, openBlacklistDialog, cancelBlacklistDialog, handleSaveBlacklist, handleToggleBlacklist } =
    useModelProvidersBlacklist({
      providers,
      blacklistDialogOpen,
      setBlacklistDialogOpen,
      blacklistedIds,
      setBlacklistedIds,
      setBlacklistLoading,
      setBlacklistSaving,
      blacklistSearchTerm,
      setBlacklistSearchTerm,
      blacklistFilter,
      setBlacklistFilter,
    });

  const { templateData, handleToggleTemplateEditor, handleAddTemplateItem, handleDeleteTemplateItem } =
    useModelProvidersTemplateEditor({
      templateEditorOpen,
      setTemplateEditorOpen,
      setTemplateLoading,
      selectedModelId,
      templateNewItem,
      setTemplateNewItem,
    });

  const buildPayload = buildAssociationPayload;

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

  const { testResults, structuredTestResults, handleTest, dialogClose, executeTestNow } = useModelProvidersTesting({
    setTestDialogOpen,
    setSelectedTestId,
    setTestType,
    setReactTestResult,
    selectedTestId,
    testType,
  });

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

  const { handleResetWeights, handleResetPriorities, handleEnableAssociations } = useModelProvidersOperationScope({
    selectedModelId,
    isGlobalScope,
    fetchModelProviders,
    setResettingWeights,
    setResettingPriorities,
    setEnablingAssociations,
  });

  const { previewData, handleAutoAssociate, handleCleanInvalid, executePreviewAction } = useModelProvidersPreview({
    selectedModelId,
    previewType,
    setPreviewType,
    setPreviewDialogOpen,
    setExecuting,
    fetchModelProviders,
  });

  const {
    handleSelectAllAssociations,
    handleSelectOneAssociation,
    handleBatchDeleteAssociations,
    handleBatchUpdateStatus,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    selectAllSuccessful,
    selectAllFailed,
    clearBatchTestResults,
  } = useModelProvidersBatch({
    selectedModelId,
    fetchModelProviders,
    filteredModelProviders,
    selectedAssociationIds,
    setSelectedAssociationIds,
    setModelProviders,
    setBatchDeleteDialogOpen,
    setBatchDeleting,
    setBatchUpdatingStatus,
    setBatchTesting,
    setBatchTestProgress,
    associationTestResults,
    setAssociationTestResults,
  });

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

