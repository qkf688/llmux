import type { FieldArrayWithId, UseFormReturn } from "react-hook-form";
import type {
  AssociationPreview,
  Model,
  ModelProviderTestResult,
  ModelTemplate,
  ModelWithProvider,
  Provider,
} from "@/lib/api";
import type { FormValues } from "../form-schema";
import type {
  BlacklistFilter,
  ProviderModelGroup,
  ProviderModelSelection,
  ProviderModelWithOwner,
  TestType,
} from "../types";

type UseModelProvidersPageDialogPropsInput = {
  // Blacklist dialog
  blacklistDialogOpen: boolean;
  onBlacklistDialogOpenChange: (open: boolean) => void;
  providers: Provider[];
  filteredProviders: Provider[];
  blacklistedIds: number[];
  blacklistLoading: boolean;
  blacklistSaving: boolean;
  blacklistSearchTerm: string;
  blacklistFilter: BlacklistFilter;
  onBlacklistSearchTermChange: (value: string) => void;
  onBlacklistFilterChange: (value: BlacklistFilter) => void;
  onToggleBlacklist: (providerId: number, checked: boolean) => void;
  onSaveBlacklist: () => void;
  onCancelBlacklist: () => void;

  // Template editor dialog
  templateEditorOpen: boolean;
  onTemplateEditorOpenChange: (open: boolean) => void;
  selectedModelId: number | null;
  templateLoading: boolean;
  templateData: ModelTemplate | null;
  templateNewItem: string;
  onTemplateNewItemChange: (value: string) => void;
  onAddTemplateItem: () => void;
  onDeleteTemplateItem: (name: string) => void;

  // Association form dialog
  associationFormOpen: boolean;
  onAssociationFormOpenChange: (open: boolean) => void;
  editingAssociation: ModelWithProvider | null;
  form: UseFormReturn<FormValues>;
  models: Model[];
  providersForForm: Provider[];
  selectedProviderModels: ProviderModelSelection[];
  isSubmitting: boolean;
  headerFields: FieldArrayWithId<FormValues, "customer_headers", "id">[];
  appendHeader: (value: { key: string; value: string }) => void;
  removeHeader: (index: number) => void;
  onSubmitCreate: (values: FormValues) => Promise<void>;
  onSubmitUpdate: (values: FormValues) => Promise<void>;
  onOpenModelListDialog: () => void;
  onClearSelectedProviderModels: () => void;
  onRemoveSelectedProviderModel: (selectionKey: string) => void;
  onProviderChange: () => void;

  // Test dialog
  testDialogOpen: boolean;
  onTestDialogOpenChange: (open: boolean) => void;
  testType: TestType;
  onTestTypeChange: (value: TestType) => void;
  selectedTestId: number | null;
  testResults: Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;
  structuredTestResults: Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;
  reactTestResult: { loading: boolean; messages: string; success: boolean | null; error: string | null };
  onCloseTestDialog: () => void;
  onExecuteTestNow: () => void;

  // Model list dialog
  modelListDialogOpen: boolean;
  onModelListDialogOpenChange: (open: boolean) => void;
  modelSearchKeyword: string;
  onModelSearchKeywordChange: (value: string) => void;
  loadingProviderModels: boolean;
  providerModels: ProviderModelWithOwner[];
  visibleProviderGroups: ProviderModelGroup[];
  visibleAvailableModels: ProviderModelWithOwner[];
  visibleExistingCount: number;
  selectedProviderModelsForList: ProviderModelSelection[];
  selectedKeys: Set<string>;
  existingAssociationKeys: Set<string>;
  collapsedProviders: Record<number, boolean>;
  onToggleProviderCollapse: (providerId: number) => void;
  onSelectAllVisibleAvailable: () => void;
  onClearModelListSelection: () => void;
  onToggleModelSelection: (model: ProviderModelWithOwner, checked: boolean, selectionKey: string) => void;

  // Preview dialog
  previewDialogOpen: boolean;
  onPreviewDialogOpenChange: (open: boolean) => void;
  previewType: "associate" | "clean";
  previewData: AssociationPreview[];
  executing: boolean;
  onConfirmPreview: () => void;
};

export function useModelProvidersPageDialogProps(input: UseModelProvidersPageDialogPropsInput) {
  const blacklistDialogProps = {
    open: input.blacklistDialogOpen,
    onOpenChange: input.onBlacklistDialogOpenChange,
    providers: input.providers,
    filteredProviders: input.filteredProviders,
    blacklistedIds: input.blacklistedIds,
    loading: input.blacklistLoading,
    saving: input.blacklistSaving,
    searchTerm: input.blacklistSearchTerm,
    filter: input.blacklistFilter,
    onSearchTermChange: input.onBlacklistSearchTermChange,
    onFilterChange: input.onBlacklistFilterChange,
    onToggle: input.onToggleBlacklist,
    onSave: input.onSaveBlacklist,
    onCancel: input.onCancelBlacklist,
  };

  const templateEditorDialogProps = {
    open: input.templateEditorOpen,
    onOpenChange: input.onTemplateEditorOpenChange,
    selectedModelId: input.selectedModelId,
    loading: input.templateLoading,
    templateData: input.templateData,
    newItem: input.templateNewItem,
    onNewItemChange: input.onTemplateNewItemChange,
    onAdd: input.onAddTemplateItem,
    onDelete: input.onDeleteTemplateItem,
  };

  const associationFormDialogProps = {
    open: input.associationFormOpen,
    onOpenChange: input.onAssociationFormOpenChange,
    editingAssociation: input.editingAssociation,
    form: input.form,
    models: input.models,
    providers: input.providersForForm,
    selectedProviderModels: input.selectedProviderModels,
    isSubmitting: input.isSubmitting,
    headerFields: input.headerFields,
    appendHeader: input.appendHeader,
    removeHeader: input.removeHeader,
    onSubmitCreate: input.onSubmitCreate,
    onSubmitUpdate: input.onSubmitUpdate,
    onOpenModelListDialog: input.onOpenModelListDialog,
    onClearSelectedProviderModels: input.onClearSelectedProviderModels,
    onRemoveSelectedProviderModel: input.onRemoveSelectedProviderModel,
    onProviderChange: input.onProviderChange,
  };

  const testDialogProps = {
    open: input.testDialogOpen,
    onOpenChange: input.onTestDialogOpenChange,
    testType: input.testType,
    onTestTypeChange: input.onTestTypeChange,
    selectedTestId: input.selectedTestId,
    testResults: input.testResults,
    structuredTestResults: input.structuredTestResults,
    reactTestResult: input.reactTestResult,
    onClose: input.onCloseTestDialog,
    onExecute: input.onExecuteTestNow,
  };

  const modelListDialogProps = {
    open: input.modelListDialogOpen,
    onOpenChange: input.onModelListDialogOpenChange,
    modelSearchKeyword: input.modelSearchKeyword,
    onModelSearchKeywordChange: input.onModelSearchKeywordChange,
    loadingProviderModels: input.loadingProviderModels,
    providerModels: input.providerModels,
    visibleProviderGroups: input.visibleProviderGroups,
    visibleAvailableModels: input.visibleAvailableModels,
    visibleExistingCount: input.visibleExistingCount,
    selectedProviderModels: input.selectedProviderModelsForList,
    selectedKeys: input.selectedKeys,
    existingAssociationKeys: input.existingAssociationKeys,
    collapsedProviders: input.collapsedProviders,
    onToggleProviderCollapse: input.onToggleProviderCollapse,
    onSelectAllVisibleAvailable: input.onSelectAllVisibleAvailable,
    onClearSelection: input.onClearModelListSelection,
    onToggleModelSelection: input.onToggleModelSelection,
  };

  const previewDialogProps = {
    open: input.previewDialogOpen,
    onOpenChange: input.onPreviewDialogOpenChange,
    type: input.previewType,
    data: input.previewData,
    executing: input.executing,
    onConfirm: input.onConfirmPreview,
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

