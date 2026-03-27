import { useState, useEffect } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
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
import type { Provider, ProviderTemplate } from "@/lib/api";
import { parseUpstreamModelsFromConfig } from "@/lib/provider-models";
import { Spinner } from "@/components/ui/spinner";
import { defaultProviderFormValues, providerFormSchema, type ProviderFormValues } from "./form-schema";
import { extractAllModels } from "./utils/config";
import { useProvidersPageStore } from "@/stores/providers";
import { useProviderModelTesting } from "./hooks/use-provider-model-testing";
import { useAllModelsDialog } from "./hooks/use-all-models-dialog";
import { useUpstreamModelsDialog } from "./hooks/use-upstream-models-dialog";
import { useProviderDialog } from "./hooks/use-provider-dialog";
import { useProviderDangerActions } from "./hooks/use-provider-danger-actions";
import { useProviderMutations } from "./hooks/use-provider-mutations";
import { useProviderSyncActions } from "./hooks/use-provider-sync-actions";
import { useProviderSwitchActions } from "./hooks/use-provider-switch-actions";
import { useProvidersBootstrap } from "./hooks/use-providers-bootstrap";
import { applyProviderTemplateDefaults } from "./utils/template-defaults";

export default function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [providerTemplates, setProviderTemplates] = useState<ProviderTemplate[]>([]);

  // 上游模型测试相关状态
  const {
    loading,
    setLoading,
    clearingAssociation,
    setClearingAssociation,
    modelsLoading,
    setModelsLoading,
    addingModels,
    setAddingModels,
    syncingModels,
    setSyncingModels,
    syncingAll,
    setSyncingAll,
    autoAssociateOnAddEnabled,
    setAutoAssociateOnAddEnabled,
    autoCleanOnDeleteEnabled,
    setAutoCleanOnDeleteEnabled,
    open,
    setOpen,
    editingProvider,
    setEditingProvider,
    deleteId,
    setDeleteId,
    clearAssociationId,
    setClearAssociationId,
    modelsOpen,
    setModelsOpen,
    modelsOpenId,
    setModelsOpenId,
    selectedUpstreamModels,
    setSelectedUpstreamModels,
    allModelsOpen,
    setAllModelsOpen,
    allModelsProvider,
    setAllModelsProvider,
    selectedAllModels,
    setSelectedAllModels,
    customModelInput,
    setCustomModelInput,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
    allModelsTestResults,
    setAllModelsTestResults,
    batchTesting,
    setBatchTesting,
    batchTestProgress,
    setBatchTestProgress,
    upstreamTestResults,
    setUpstreamTestResults,
    upstreamBatchTesting,
    setUpstreamBatchTesting,
    upstreamBatchTestProgress,
    setUpstreamBatchTestProgress,
    showApiKey,
    setShowApiKey,
    toggleShowApiKey,
    nameFilter,
    setNameFilter,
    debouncedNameFilter,
    setDebouncedNameFilter,
    typeFilter,
    setTypeFilter,
    availableTypes,
    setAvailableTypes,
    flushNameFilter,
    resetTransient,
  } = useProvidersPageStore((state) => ({
    loading: state.loading,
    setLoading: state.setLoading,
    clearingAssociation: state.clearingAssociation,
    setClearingAssociation: state.setClearingAssociation,
    modelsLoading: state.modelsLoading,
    setModelsLoading: state.setModelsLoading,
    addingModels: state.addingModels,
    setAddingModels: state.setAddingModels,
    syncingModels: state.syncingModels,
    setSyncingModels: state.setSyncingModels,
    syncingAll: state.syncingAll,
    setSyncingAll: state.setSyncingAll,
    autoAssociateOnAddEnabled: state.autoAssociateOnAddEnabled,
    setAutoAssociateOnAddEnabled: state.setAutoAssociateOnAddEnabled,
    autoCleanOnDeleteEnabled: state.autoCleanOnDeleteEnabled,
    setAutoCleanOnDeleteEnabled: state.setAutoCleanOnDeleteEnabled,
    open: state.providerDialogOpen,
    setOpen: state.setProviderDialogOpen,
    editingProvider: state.editingProvider,
    setEditingProvider: state.setEditingProvider,
    deleteId: state.deleteId,
    setDeleteId: state.setDeleteId,
    clearAssociationId: state.clearAssociationId,
    setClearAssociationId: state.setClearAssociationId,
    modelsOpen: state.modelsOpen,
    setModelsOpen: state.setModelsOpen,
    modelsOpenId: state.modelsOpenId,
    setModelsOpenId: state.setModelsOpenId,
    selectedUpstreamModels: state.selectedUpstreamModels,
    setSelectedUpstreamModels: state.setSelectedUpstreamModels,
    allModelsOpen: state.allModelsOpen,
    setAllModelsOpen: state.setAllModelsOpen,
    allModelsProvider: state.allModelsProvider,
    setAllModelsProvider: state.setAllModelsProvider,
    selectedAllModels: state.selectedAllModels,
    setSelectedAllModels: state.setSelectedAllModels,
    customModelInput: state.customModelInput,
    setCustomModelInput: state.setCustomModelInput,
    allModelsSearchQuery: state.allModelsSearchQuery,
    setAllModelsSearchQuery: state.setAllModelsSearchQuery,
    allModelsTestResults: state.allModelsTestResults,
    setAllModelsTestResults: state.setAllModelsTestResults,
    batchTesting: state.batchTesting,
    setBatchTesting: state.setBatchTesting,
    batchTestProgress: state.batchTestProgress,
    setBatchTestProgress: state.setBatchTestProgress,
    upstreamTestResults: state.upstreamTestResults,
    setUpstreamTestResults: state.setUpstreamTestResults,
    upstreamBatchTesting: state.upstreamBatchTesting,
    setUpstreamBatchTesting: state.setUpstreamBatchTesting,
    upstreamBatchTestProgress: state.upstreamBatchTestProgress,
    setUpstreamBatchTestProgress: state.setUpstreamBatchTestProgress,
    showApiKey: state.showApiKey,
    setShowApiKey: state.setShowApiKey,
    toggleShowApiKey: state.toggleShowApiKey,
    nameFilter: state.nameFilter,
    setNameFilter: state.setNameFilter,
    debouncedNameFilter: state.debouncedNameFilter,
    setDebouncedNameFilter: state.setDebouncedNameFilter,
    typeFilter: state.typeFilter,
    setTypeFilter: state.setTypeFilter,
    availableTypes: state.availableTypes,
    setAvailableTypes: state.setAvailableTypes,
    flushNameFilter: state.flushNameFilter,
    resetTransient: state.resetTransient,
  }));

  // 筛选条件
  useEffect(() => {
    return () => {
      resetTransient();
    };
  }, [resetTransient]);

  // 初始化表单
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: { ...defaultProviderFormValues },
  });

  // 监听类型变化，用于显示/隐藏 Anthropic 特有字段
  const watchedType = form.watch("type");

  useEffect(() => {
    const timeoutId = window.setTimeout(() => {
      setDebouncedNameFilter(nameFilter);
    }, 250);
    return () => window.clearTimeout(timeoutId);
  }, [nameFilter, setDebouncedNameFilter]);

  const { fetchProviders } = useProvidersBootstrap({
    debouncedNameFilter,
    typeFilter,
    setLoading,
    setProviders,
    setProviderTemplates,
    setAvailableTypes,
    setAutoAssociateOnAddEnabled,
    setAutoCleanOnDeleteEnabled,
  });

  const autoActionsFlags = { autoAssociateOnAddEnabled, autoCleanOnDeleteEnabled };

  const getAllModelsForProvider = (providerId: number): string[] => {
    const provider = providers.find((item) => item.ID === providerId);
    if (!provider) return [];
    return extractAllModels(provider.Config);
  };

  const {
    allModelsList,
    filteredAllModels,
    setAllModelsList,
    upstreamModelsList,
    upstreamStatus,
    openAllModelsDialog,
    persistModels,
    handleAddCustomModels,
    handleRemoveModelFromAll,
    handleRemoveSelectedModels,
    handleSyncUpstreamModels,
  } = useAllModelsDialog({
    setProviders,
    fetchProviders,
    allModelsProvider,
    setAllModelsProvider,
    setAllModelsOpen,
    selectedAllModels,
    setSelectedAllModels,
    customModelInput,
    setCustomModelInput,
    allModelsSearchQuery,
    setAllModelsSearchQuery,
    setAllModelsTestResults,
    setAddingModels,
    setSyncingModels,
    autoActionsFlags,
  });

  const {
    providerModels,
    filteredProviderModels,
    openModelsDialog,
    refreshUpstreamModels,
    handleUpstreamSearchChange,
    handleAddUpstreamToAll,
  } = useUpstreamModelsDialog({
    providers,
    modelsOpenId,
    setModelsOpen,
    setModelsOpenId,
    modelsLoading,
    setModelsLoading,
    addingModels,
    setAddingModels,
    selectedUpstreamModels,
    setSelectedUpstreamModels,
    allModelsProvider,
    setAllModelsProvider,
    setAllModelsList,
    persistModels,
    autoActionsFlags,
  });

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

  const {
    updatingFilter,
    updatingAssociationTrigger,
    handleToggleModelEndpoint,
    handleToggleModelFilter,
    handleToggleAssociationTrigger,
  } = useProviderSwitchActions({ setProviders });


  const {
    copyModelName,
    handleTestAllModel,
    selectAllSuccessful,
    selectAllFailed,
    handleBatchTestAll,
    handleBatchTestSelected,
    handleCancelBatchTest,
    handleTestUpstreamModel,
    handleBatchTestUpstreamAll,
    handleBatchTestUpstreamSelected,
    handleCancelUpstreamBatchTest,
    selectUpstreamSuccessful,
    selectUpstreamFailed,
  } = useProviderModelTesting({
    allModelsProvider,
    modelsOpenId,
    filteredAllModels,
    filteredProviderModels,
    selectedAllModels,
    selectedUpstreamModels,
    allModelsTestResults,
    upstreamTestResults,
    setSelectedAllModels,
    setSelectedUpstreamModels,
    setAllModelsTestResults,
    setUpstreamTestResults,
    setBatchTesting,
    setBatchTestProgress,
    setUpstreamBatchTesting,
    setUpstreamBatchTestProgress,
  });

  const { handleSyncAllProviders } = useProviderSyncActions({ setSyncingAll, fetchProviders });

  const { handleSubmitProvider } = useProviderMutations({
    form,
    editingProvider,
    setEditingProvider,
    setOpen,
    fetchProviders,
  });

  const { openEditDialog, openCreateDialog } = useProviderDialog({
    form,
    setOpen,
    setEditingProvider,
    setShowApiKey,
  });

  const {
    openDeleteDialog,
    cancelDeleteDialog,
    handleDelete,
    openClearAssociationsDialog,
    cancelClearAssociationsDialog,
    handleClearAssociations,
  } = useProviderDangerActions({
    providers,
    fetchProviders,
    deleteId,
    setDeleteId,
    clearAssociationId,
    setClearAssociationId,
    clearingAssociation,
    setClearingAssociation,
    autoCleanOnDeleteEnabled,
  });

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
           <div className="flex flex-col gap-1 text-xs col-span-1">
             <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商名称</Label>
             <Input
               placeholder="输入名称"
               value={nameFilter}
               onChange={(e) => setNameFilter(e.target.value)}
               className="h-8 w-full text-xs px-2"
             />
           </div>
           <div className="flex flex-col gap-1 text-xs col-span-1">
             <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">类型</Label>
             <Select
               value={typeFilter}
                onValueChange={(value) => {
                  setTypeFilter(value);
                  flushNameFilter();
                }}
              >
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
              <Table className="min-w-[1200px]">
                <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>名称</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>全部模型</TableHead>
                    <TableHead>模型端点</TableHead>
                    <TableHead>关联触发</TableHead>
                    <TableHead className="w-[360px]">操作</TableHead>
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
                          <Switch
                            checked={!(provider.blacklisted ?? false)}
                            onCheckedChange={(checked) => handleToggleAssociationTrigger(provider, checked)}
                            disabled={updatingAssociationTrigger[provider.ID]}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2 items-center">
                            <TooltipProvider>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <div className="flex items-center">
                                    <Switch
                                      checked={provider.ModelFilterEnabled ?? false}
                                      onCheckedChange={(checked) => handleToggleModelFilter(provider, checked)}
                                      disabled={updatingFilter[provider.ID]}
                                    />
                                  </div>
                                </TooltipTrigger>
                                <TooltipContent>启用模型过滤</TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                            <Button variant="outline" size="sm" onClick={() => openEditDialog(provider)}>
                              编辑
                            </Button>
                            <Button variant="secondary" size="sm" onClick={() => openModelsDialog(provider.ID)}>
                              获取模型
                            </Button>
                            <AlertDialog>
                              <AlertDialogTrigger asChild>
                                <Button variant="outline" size="sm" onClick={() => openClearAssociationsDialog(provider.ID)}>
                                  清除关联
                                </Button>
                              </AlertDialogTrigger>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>确定要清除这个提供商的所有关联吗？</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    此操作将删除该提供商下所有的模型关联关系，但不会删除提供商本身。此操作无法撤销。
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel onClick={cancelClearAssociationsDialog}>取消</AlertDialogCancel>
                                  <AlertDialogAction 
                                    onClick={handleClearAssociations} 
                                    disabled={clearingAssociation}
                                    className="bg-destructive hover:bg-destructive/90"
                                  >
                                    {clearingAssociation ? "清除中..." : "确认清除"}
                                  </AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
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
                                  <AlertDialogCancel onClick={cancelDeleteDialog}>取消</AlertDialogCancel>
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
            <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border">
              {providers.map((provider) => {
                const allModels = extractAllModels(provider.Config);
                return (
                  <div key={provider.ID} className="py-2 space-y-2">
                    <div className="min-w-0">
                      <h3 className="font-semibold text-[13px] leading-snug whitespace-normal break-all">
                        {provider.Name}
                      </h3>
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 pt-0.5 text-[10px] text-muted-foreground leading-tight">
                        <span className="shrink-0">ID: {provider.ID}</span>
                        <span className="shrink-0">类型: {provider.Type || "未知"}</span>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-6 px-2 text-[11px] gap-1.5"
                        onClick={() => openAllModelsDialog(provider)}
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-3 w-3">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                        </svg>
                        {allModels.length}
                      </Button>

                      <div className="flex items-center gap-1.5">
                        <span className="text-[10px] text-muted-foreground">端点</span>
                        <Switch
                          checked={provider.ModelEndpoint ?? true}
                          onCheckedChange={() => handleToggleModelEndpoint(provider)}
                          className="scale-75"
                        />
                      </div>

                      <div className="flex items-center gap-1.5">
                        <span className="text-[10px] text-muted-foreground">关联</span>
                        <Switch
                          checked={!(provider.blacklisted ?? false)}
                          onCheckedChange={(checked) => handleToggleAssociationTrigger(provider, checked)}
                          disabled={updatingAssociationTrigger[provider.ID]}
                          className="scale-75"
                        />
                      </div>

                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div className="flex items-center gap-1.5">
                              <span className="text-[10px] text-muted-foreground">过滤</span>
                              <Switch
                                checked={provider.ModelFilterEnabled ?? false}
                                onCheckedChange={(checked) => handleToggleModelFilter(provider, checked)}
                                disabled={updatingFilter[provider.ID]}
                                className="scale-75"
                              />
                            </div>
                          </TooltipTrigger>
                          <TooltipContent>启用模型过滤</TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>

                    <div className="flex flex-wrap justify-end gap-1.5 pt-0.5">
                        <Button variant="outline" size="sm" className="h-6 px-2 text-[11px]" onClick={() => openEditDialog(provider)}>
                          编辑
                        </Button>
                        <Button variant="secondary" size="sm" className="h-6 px-2 text-[11px]" onClick={() => openModelsDialog(provider.ID)}>
                          模型
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="outline" size="sm" className="h-6 px-2 text-[11px]" onClick={() => openClearAssociationsDialog(provider.ID)}>
                              清除关联
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>确定要清除这个提供商的所有关联吗？</AlertDialogTitle>
                              <AlertDialogDescription>
                                此操作将删除该提供商下所有的模型关联关系，但不会删除提供商本身。此操作无法撤销。
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel onClick={cancelClearAssociationsDialog}>取消</AlertDialogCancel>
                              <AlertDialogAction 
                                onClick={handleClearAssociations} 
                                disabled={clearingAssociation}
                                className="bg-destructive hover:bg-destructive/90"
                              >
                                {clearingAssociation ? "清除中..." : "确认清除"}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="destructive" size="sm" className="h-6 px-2 text-[11px]" onClick={() => openDeleteDialog(provider.ID)}>
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
                              <AlertDialogCancel onClick={cancelDeleteDialog}>取消</AlertDialogCancel>
                              <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
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
          <DialogHeader className="flex-shrink-0">
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
            <form
              onSubmit={form.handleSubmit(handleSubmitProvider)}
              className="space-y-4 min-w-0 overflow-y-auto flex-1 min-h-0 -mx-1 px-1"
            >
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
                          applyProviderTemplateDefaults(e.target.value, providerTemplates, form);
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
                      <Input
                        {...field}
                        autoComplete="url"
                        inputMode="url"
                        placeholder="https://api.openai.com/v1"
                      />
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
                          autoComplete="new-password"
                          placeholder="sk-..."
                          className="pr-10"
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="absolute right-1.5 top-1/2 -translate-y-1/2 h-8 w-8"
                          onClick={toggleShowApiKey}
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

                  <FormField
                    control={form.control}
                    name="auth_type"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>认证方式</FormLabel>
                        <Select onValueChange={field.onChange} value={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="选择认证方式" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="x-api-key">x-api-key（Anthropic 官方）</SelectItem>
                            <SelectItem value="bearer">Authorization: Bearer（兼容第三方）</SelectItem>
                          </SelectContent>
                        </Select>
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

              <FormField
                control={form.control}
                name="model_filter_enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border p-3">
                    <div className="space-y-0.5">
                      <FormLabel>启用模型过滤</FormLabel>
                      <div className="text-sm text-muted-foreground">
                        同步时只保留符合过滤规则的模型
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
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="secondary"
                          size="icon"
                          className="h-8 w-8"
                          onClick={handleSyncUpstreamModels}
                          disabled={syncingModels || batchTesting || !allModelsProvider}
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
        <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
          <DialogHeader className="flex-shrink-0">
            <DialogTitle>{providers.find(v => v.ID === modelsOpenId)?.Name} 上游模型</DialogTitle>
            <DialogDescription>
              从上游拉取的模型列表，勾选后可加入"全部模型"缓存。
            </DialogDescription>
          </DialogHeader>

          {/* 测试结果统计 */}
          {Object.keys(upstreamTestResults).length > 0 && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground flex-shrink-0">
              <span>已测试: {Object.keys(upstreamTestResults).length}</span>
              <span className="text-muted-foreground">|</span>
              <span className="text-green-600">
                成功: {Object.values(upstreamTestResults).filter(r => r.success === true).length}
              </span>
              <span className="text-muted-foreground">|</span>
              <span className="text-red-600">
                失败: {Object.values(upstreamTestResults).filter(r => r.success === false).length}
              </span>
            </div>
          )}

          {/* 批量测试进度条 */}
          {upstreamBatchTesting && (
            <div className="flex items-center gap-3 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md flex-shrink-0">
              <div className="flex-1">
                <div className="flex items-center justify-between text-xs text-blue-800 mb-1">
                  <span>
                    测试进度：{upstreamBatchTestProgress.completed}/{upstreamBatchTestProgress.total}
                    (成功: {upstreamBatchTestProgress.success}, 失败: {upstreamBatchTestProgress.failed}, 进行中: {upstreamBatchTestProgress.testing})
                  </span>
                  <span>{Math.round((upstreamBatchTestProgress.completed / upstreamBatchTestProgress.total) * 100)}%</span>
                </div>
                <div className="w-full bg-blue-200 rounded-full h-2">
                  <div
                    className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${(upstreamBatchTestProgress.completed / upstreamBatchTestProgress.total) * 100}%` }}
                  />
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={handleCancelUpstreamBatchTest}
                className="h-7 text-xs"
              >
                取消
              </Button>
            </div>
          )}

          <div className="flex items-center justify-between gap-2 flex-shrink-0">
            <div className="text-sm text-muted-foreground">
              {modelsLoading
                ? "正在从上游获取..."
                : `上游返回 ${providerModels.length} 个，已缓存 ${getAllModelsForProvider(modelsOpenId || 0).length} 个`}
            </div>
            <div className="flex gap-1 flex-wrap">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="default"
                      size="icon"
                      className="h-8 w-8"
                      onClick={handleBatchTestUpstreamAll}
                      disabled={filteredProviderModels.length === 0 || upstreamBatchTesting || addingModels}
                    >
                      {upstreamBatchTesting ? (
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
                      onClick={handleBatchTestUpstreamSelected}
                      disabled={selectedUpstreamModels.length === 0 || upstreamBatchTesting || addingModels}
                    >
                      <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                      </svg>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>批量测试选中的 {selectedUpstreamModels.length} 个模型</TooltipContent>
                </Tooltip>
              </TooltipProvider>

              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-8 w-8"
                      onClick={selectUpstreamSuccessful}
                      disabled={Object.values(upstreamTestResults).filter(r => r.success === true).length === 0 || upstreamBatchTesting}
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
                      onClick={selectUpstreamFailed}
                      disabled={Object.values(upstreamTestResults).filter(r => r.success === false).length === 0 || upstreamBatchTesting}
                    >
                      <svg className="h-4 w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>选择测试失败的模型</TooltipContent>
                </Tooltip>
              </TooltipProvider>

              <Button
                variant="outline"
                size="sm"
                onClick={toggleSelectAll}
                disabled={selectableModelIds.length === 0 || upstreamBatchTesting}
              >
                {isAllSelectableChecked ? "取消全选" : "全选可添加"}
              </Button>
              <Button variant="outline" size="sm" onClick={refreshUpstreamModels} disabled={!modelsOpenId || modelsLoading || upstreamBatchTesting}>
                {modelsLoading ? "刷新中" : "刷新上游"}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleAddUpstreamToAll}
                disabled={selectedUpstreamModels.length === 0 || addingModels || upstreamBatchTesting}
              >
                {addingModels ? "同步中..." : `添加到全部模型${selectedUpstreamModels.length > 0 ? `（${selectedUpstreamModels.length}）` : ""}`}
              </Button>
            </div>
          </div>

          {!modelsLoading && providerModels.length > 0 && (
            <div className="mb-3">
              <Input
                placeholder="搜索模型 ID"
                onChange={(e) => handleUpstreamSearchChange(e.target.value)}
                className="w-full"
              />
            </div>
          )}

          {modelsLoading ? (
            <Loading message="加载模型列表" />
          ) : (
            <div className="max-h-96 overflow-y-auto space-y-2 flex-1 min-h-0">
              {filteredProviderModels.length === 0 ? (
                <div className="text-center text-gray-500 py-8">
                  {providerModels.length === 0 ? '暂无模型数据' : '未找到匹配的模型'}
                </div>
              ) : (
                (() => {
                  return filteredProviderModels.map((model) => {
                    const isSaved = savedModelSet.has(model.id.toLowerCase());
                    const checked = selectedUpstreamModels.includes(model.id);
                    const testResult = upstreamTestResults[model.id];
                    return (
                      <div
                        key={model.id}
                        className={`flex items-center justify-between p-2 border rounded-lg ${
                          isSaved ? "border-gray-300 bg-gray-50/50" : checked ? "bg-blue-50/80 border-blue-200" : "border-border bg-background"
                        }`}
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <Checkbox
                            checked={checked}
                            disabled={isSaved || upstreamBatchTesting}
                            onCheckedChange={(value) => {
                              if (value) {
                                setSelectedUpstreamModels((prev) => Array.from(new Set([...prev, model.id])));
                              } else {
                                setSelectedUpstreamModels((prev) => prev.filter((item) => item !== model.id));
                              }
                            }}
                          />
                          <div className="min-w-0 flex-1">
                            <div className={`font-medium truncate ${isSaved ? "text-muted-foreground/70" : ""}`}>
                              {model.id}
                            </div>
                            {isSaved && (
                              <span className="text-xs text-muted-foreground">已缓存</span>
                            )}
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
                                  onClick={() => handleTestUpstreamModel(model.id)}
                                  disabled={!!testResult?.loading || upstreamBatchTesting}
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
                                  variant="outline"
                                  size="sm"
                                  onClick={() => copyModelName(model.id)}
                                  className="h-7 px-2"
                                >
                                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" aria-hidden="true" className="h-3.5 w-3.5"><path strokeLinecap="round" strokeLinejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path></svg>
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>复制名称</TooltipContent>
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
 
