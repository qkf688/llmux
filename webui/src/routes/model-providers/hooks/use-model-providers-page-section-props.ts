import type { Model, ModelWithProvider, Provider } from "@/lib/api";
import type { AssociationBatchTestResult, BatchTestProgress } from "../types";

type UseModelProvidersPageSectionPropsInput = {
  operationScope: "current" | "all";
  onOperationScopeChange: (scope: "current" | "all") => void;
  selectedModelName: string;
  selectedModelId: number | null;
  resettingWeights: boolean;
  resettingPriorities: boolean;
  enablingAssociations: boolean;
  onResetWeights: () => void;
  onResetPriorities: () => void;
  onEnableAssociations: () => void;

  filterPanelOpen: boolean;
  onFilterPanelOpenChange: (open: boolean) => void;
  activeFilterCount: number;
  models: Model[];
  onModelChange: (modelId: string) => void;
  selectedProviderType: string;
  onSelectedProviderTypeChange: (value: string) => void;
  selectedProviderFilter: string;
  onSelectedProviderFilterChange: (value: string) => void;
  selectedStatusFilter: string;
  onSelectedStatusFilterChange: (value: string) => void;
  searchKeyword: string;
  onSearchKeywordChange: (value: string) => void;
  providers: Provider[];
  providerTypes: string[];
  selectedAssociationCount: number;
  batchUpdatingStatus: boolean;
  batchActionSheetOpen: boolean;
  onBatchActionSheetOpenChange: (open: boolean) => void;
  batchCapabilitiesDialogOpen: boolean;
  onBatchCapabilitiesDialogOpenChange: (open: boolean) => void;
  batchUpdatingCapabilities: boolean;
  onBatchUpdateCapabilities: (capabilities: { tool_call?: boolean; structured_output?: boolean; image?: boolean }) => Promise<void>;
  batchTesting: boolean;
  filteredAssociationCount: number;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  batchDeleteDialogOpen: boolean;
  onBatchDeleteDialogOpenChange: (open: boolean) => void;
  batchDeleting: boolean;
  onBatchDeleteConfirm: () => void;
  onBatchUpdateStatus: (status: boolean) => Promise<void>;
  onBatchTestSelected: () => Promise<void>;
  onBatchTestAll: () => Promise<void>;
  onSelectAllSuccessful: () => void;
  onSelectAllFailed: () => void;
  onToggleTemplateEditor: () => void;
  onOpenBlacklistDialog: () => void;
  onAutoAssociate: () => Promise<void>;
  onCleanInvalid: () => Promise<void>;
  onOpenCreateDialog: () => void;

  batchTestProgress: BatchTestProgress;
  onCancelBatchTest: () => void;
  onClearBatchTestResults: () => void;

  loading: boolean;
  hasAssociationFilter: boolean;
  associations: ModelWithProvider[];
  selectedAssociationIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  providerStatus: Record<number, boolean[]>;
  healthStatus: Record<number, boolean[]>;
  statusUpdating: Record<number, boolean>;
  deleteId: number | null;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onRefreshStatus: () => void;
  onToggleStatus: (association: ModelWithProvider, nextStatus: boolean) => void;
  onEdit: (association: ModelWithProvider) => void;
  onOpenDelete: (id: number) => void;
  onDeleteDialogChange: (openValue: boolean) => void;
  onDeleteConfirm: () => void;
  onTest: (id: number) => void;
};

export function useModelProvidersPageSectionProps(input: UseModelProvidersPageSectionPropsInput) {
  const operationScopeToolbarProps = {
    operationScope: input.operationScope,
    onOperationScopeChange: input.onOperationScopeChange,
    selectedModelName: input.selectedModelName,
    selectedModelId: input.selectedModelId,
    resettingWeights: input.resettingWeights,
    resettingPriorities: input.resettingPriorities,
    enablingAssociations: input.enablingAssociations,
    onResetWeights: input.onResetWeights,
    onResetPriorities: input.onResetPriorities,
    onEnableAssociations: input.onEnableAssociations,
  };

  const associationFilterPanelProps = {
    filterPanelOpen: input.filterPanelOpen,
    onFilterPanelOpenChange: input.onFilterPanelOpenChange,
    activeFilterCount: input.activeFilterCount,
    selectedModelId: input.selectedModelId,
    models: input.models,
    onModelChange: input.onModelChange,
    selectedProviderType: input.selectedProviderType,
    onSelectedProviderTypeChange: input.onSelectedProviderTypeChange,
    selectedProviderFilter: input.selectedProviderFilter,
    onSelectedProviderFilterChange: input.onSelectedProviderFilterChange,
    selectedStatusFilter: input.selectedStatusFilter,
    onSelectedStatusFilterChange: input.onSelectedStatusFilterChange,
    searchKeyword: input.searchKeyword,
    onSearchKeywordChange: input.onSearchKeywordChange,
    providers: input.providers,
    providerTypes: input.providerTypes,
    selectedAssociationCount: input.selectedAssociationCount,
    batchUpdatingStatus: input.batchUpdatingStatus,
    batchActionSheetOpen: input.batchActionSheetOpen,
    onBatchActionSheetOpenChange: input.onBatchActionSheetOpenChange,
    batchCapabilitiesDialogOpen: input.batchCapabilitiesDialogOpen,
    onBatchCapabilitiesDialogOpenChange: input.onBatchCapabilitiesDialogOpenChange,
    batchUpdatingCapabilities: input.batchUpdatingCapabilities,
    onBatchUpdateCapabilities: input.onBatchUpdateCapabilities,
    batchTesting: input.batchTesting,
    filteredAssociationCount: input.filteredAssociationCount,
    associationTestResults: input.associationTestResults,
    batchDeleteDialogOpen: input.batchDeleteDialogOpen,
    onBatchDeleteDialogOpenChange: input.onBatchDeleteDialogOpenChange,
    batchDeleting: input.batchDeleting,
    onBatchDeleteConfirm: input.onBatchDeleteConfirm,
    onBatchUpdateStatus: input.onBatchUpdateStatus,
    onBatchTestSelected: input.onBatchTestSelected,
    onBatchTestAll: input.onBatchTestAll,
    onSelectAllSuccessful: input.onSelectAllSuccessful,
    onSelectAllFailed: input.onSelectAllFailed,
    onToggleTemplateEditor: input.onToggleTemplateEditor,
    onOpenBlacklistDialog: input.onOpenBlacklistDialog,
    onAutoAssociate: input.onAutoAssociate,
    onCleanInvalid: input.onCleanInvalid,
    onOpenCreateDialog: input.onOpenCreateDialog,
  };

  const batchTestProgressCardProps = {
    batchTesting: input.batchTesting,
    batchTestProgress: input.batchTestProgress,
    associationTestResults: input.associationTestResults,
    onCancel: input.onCancelBatchTest,
    onClear: input.onClearBatchTestResults,
    onSelectSuccess: input.onSelectAllSuccessful,
    onSelectFailed: input.onSelectAllFailed,
  };

  const associationListSectionProps = {
    loading: input.loading,
    selectedModelId: input.selectedModelId,
    hasAssociationFilter: input.hasAssociationFilter,
    associations: input.associations,
    providers: input.providers,
    selectedAssociationIds: input.selectedAssociationIds,
    isAllSelected: input.isAllSelected,
    isPartialSelected: input.isPartialSelected,
    providerStatus: input.providerStatus,
    healthStatus: input.healthStatus,
    statusUpdating: input.statusUpdating,
    associationTestResults: input.associationTestResults,
    deleteId: input.deleteId,
    onSelectAll: input.onSelectAll,
    onSelectOne: input.onSelectOne,
    onRefreshStatus: input.onRefreshStatus,
    onToggleStatus: input.onToggleStatus,
    onEdit: input.onEdit,
    onOpenDelete: input.onOpenDelete,
    onDeleteDialogChange: input.onDeleteDialogChange,
    onDeleteConfirm: input.onDeleteConfirm,
    onTest: input.onTest,
  };

  return {
    operationScopeToolbarProps,
    associationFilterPanelProps,
    batchTestProgressCardProps,
    associationListSectionProps,
  };
}
