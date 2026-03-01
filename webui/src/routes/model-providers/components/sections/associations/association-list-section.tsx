import type { AssociationBatchTestResult } from "../../../types";
import type { ModelWithProvider, Provider } from "@/lib/api";
import Loading from "@/components/loading";
import { DesktopAssociationTable } from "./desktop-association-table";
import { MobileAssociationList } from "./mobile-association-list";

type AssociationListSectionProps = {
  loading: boolean;
  selectedModelId: number | null;
  hasAssociationFilter: boolean;
  associations: ModelWithProvider[];
  providers: Provider[];
  selectedAssociationIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  providerStatus: Record<number, boolean[]>;
  healthStatus: Record<number, boolean[]>;
  statusUpdating: Record<number, boolean>;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  deleteId: number | null;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onOpenBatchActionSheet: () => void;
  onRefreshStatus: () => void;
  onToggleStatus: (association: ModelWithProvider, nextStatus: boolean) => void;
  onEdit: (association: ModelWithProvider) => void;
  onOpenDelete: (id: number) => void;
  onDeleteDialogChange: (open: boolean) => void;
  onDeleteConfirm: () => void;
  onTest: (id: number) => void;
};

export function AssociationListSection({
  loading,
  selectedModelId,
  hasAssociationFilter,
  associations,
  providers,
  selectedAssociationIds,
  isAllSelected,
  isPartialSelected,
  providerStatus,
  healthStatus,
  statusUpdating,
  associationTestResults,
  deleteId,
  onSelectAll,
  onSelectOne,
  onOpenBatchActionSheet,
  onRefreshStatus,
  onToggleStatus,
  onEdit,
  onOpenDelete,
  onDeleteDialogChange,
  onDeleteConfirm,
  onTest,
}: AssociationListSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载关联数据" />
        </div>
      ) : !selectedModelId ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">
          请选择一个模型来查看其提供商关联
        </div>
      ) : associations.length === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm text-center px-6">
          {hasAssociationFilter ? "没有匹配的关联记录" : "该模型还没有关联的提供商"}
        </div>
      ) : (
        <div className="h-full flex flex-col">
          <DesktopAssociationTable
            associations={associations}
            providers={providers}
            selectedAssociationIds={selectedAssociationIds}
            isAllSelected={isAllSelected}
            isPartialSelected={isPartialSelected}
            providerStatus={providerStatus}
            healthStatus={healthStatus}
            statusUpdating={statusUpdating}
            associationTestResults={associationTestResults}
            deleteId={deleteId}
            onSelectAll={onSelectAll}
            onSelectOne={onSelectOne}
            onRefreshStatus={onRefreshStatus}
            onToggleStatus={onToggleStatus}
            onEdit={onEdit}
            onOpenDelete={onOpenDelete}
            onDeleteDialogChange={onDeleteDialogChange}
            onDeleteConfirm={onDeleteConfirm}
            onTest={onTest}
          />
          <MobileAssociationList
            associations={associations}
            providers={providers}
            selectedAssociationIds={selectedAssociationIds}
            isAllSelected={isAllSelected}
            isPartialSelected={isPartialSelected}
            providerStatus={providerStatus}
            healthStatus={healthStatus}
            statusUpdating={statusUpdating}
            associationTestResults={associationTestResults}
            deleteId={deleteId}
            onSelectAll={onSelectAll}
            onSelectOne={onSelectOne}
            onOpenBatchActionSheet={onOpenBatchActionSheet}
            onToggleStatus={onToggleStatus}
            onEdit={onEdit}
            onOpenDelete={onOpenDelete}
            onDeleteDialogChange={onDeleteDialogChange}
            onDeleteConfirm={onDeleteConfirm}
            onTest={onTest}
          />
        </div>
      )}
    </div>
  );
}

