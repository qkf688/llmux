import type { DialogPropsContext } from "./use-model-providers-page-context";

/**
 * Dialog props 装配：把上下文对象组装成 6 组对话框 props。
 * 入参为上下文对象（学 models 的 ReturnType 模式），不再接收 80+ 扁平字段。
 */
export function useModelProvidersPageDialogProps(ctx: DialogPropsContext) {
  const { store, localState, form, blacklist, templateEditor, mutations, testing, modelListVisibility, modelListSelection, preview, pageActions } = ctx;

  const blacklistDialogProps = {
    open: store.blacklistDialogOpen,
    onOpenChange: store.setBlacklistDialogOpen,
    providers: localState.providers,
    filteredProviders: blacklist.filteredProviders,
    blacklistedIds: store.blacklistedIds,
    loading: store.blacklistLoading,
    saving: store.blacklistSaving,
    searchTerm: store.blacklistSearchTerm,
    filter: store.blacklistFilter,
    onSearchTermChange: store.setBlacklistSearchTerm,
    onFilterChange: store.setBlacklistFilter,
    onToggle: blacklist.handleToggleBlacklist,
    onSave: blacklist.handleSaveBlacklist,
    onCancel: blacklist.cancelBlacklistDialog,
  };

  const templateEditorDialogProps = {
    open: store.templateEditorOpen,
    onOpenChange: store.setTemplateEditorOpen,
    selectedModelId: localState.selectedModelId,
    loading: store.templateLoading,
    templateData: templateEditor.templateData,
    newItem: store.templateNewItem,
    onNewItemChange: store.setTemplateNewItem,
    onAdd: pageActions.addTemplateItem,
    onDelete: pageActions.deleteTemplateItem,
  };

  const associationFormDialogProps = {
    open: store.open,
    onOpenChange: store.setOpen,
    editingAssociation: store.editingAssociation,
    form: form.form,
    models: localState.models,
    providers: localState.providers,
    selectedProviderModels: store.selectedProviderModels,
    isSubmitting: store.isSubmitting,
    headerFields: form.headerFields,
    appendHeader: form.appendHeader,
    removeHeader: form.removeHeader,
    onSubmitCreate: mutations.handleCreate,
    onSubmitUpdate: mutations.handleUpdate,
    onOpenModelListDialog: modelListSelection.openModelListDialog,
    onClearSelectedProviderModels: modelListSelection.clearSelectedProviderModels,
    onRemoveSelectedProviderModel: modelListSelection.removeSelectedProviderModel,
    onProviderChange: modelListSelection.handleProviderChange,
  };

  const testDialogProps = {
    open: store.testDialogOpen,
    onOpenChange: store.setTestDialogOpen,
    testType: store.testType,
    onTestTypeChange: store.setTestType,
    selectedTestId: store.selectedTestId,
    testResults: testing.testResults,
    structuredTestResults: testing.structuredTestResults,
    reactTestResult: store.reactTestResult,
    onClose: testing.dialogClose,
    onExecute: testing.executeTestNow,
  };

  const modelListDialogProps = {
    open: store.modelListDialogOpen,
    onOpenChange: store.setModelListDialogOpen,
    modelSearchKeyword: store.modelSearchKeyword,
    onModelSearchKeywordChange: store.setModelSearchKeyword,
    loadingProviderModels: localState.loading,
    providerModels: localState.providerModels,
    visibleProviderGroups: modelListVisibility.visibleProviderGroups,
    visibleAvailableModels: modelListVisibility.visibleAvailableModels,
    visibleExistingCount: modelListVisibility.visibleExistingCount,
    selectedProviderModels: store.selectedProviderModels,
    selectedKeys: modelListVisibility.selectedKeys,
    existingAssociationKeys: modelListVisibility.existingAssociationKeys,
    collapsedProviders: store.collapsedProviders,
    onToggleProviderCollapse: pageActions.toggleProviderCollapse,
    onSelectAllVisibleAvailable: modelListSelection.selectAllVisibleAvailable,
    onClearSelection: modelListSelection.clearModelListSelection,
    onToggleModelSelection: modelListSelection.toggleModelSelection,
  };

  const previewDialogProps = {
    open: store.previewDialogOpen,
    onOpenChange: store.setPreviewDialogOpen,
    type: store.previewType,
    data: preview.previewData,
    executing: store.executing,
    onConfirm: pageActions.confirmPreviewAction,
  };

  return {
    blacklistDialogProps,
    templateEditorDialogProps,
    associationFormDialogProps,
    testDialogProps,
    modelListDialogProps,
    previewDialogProps,
  };
}
