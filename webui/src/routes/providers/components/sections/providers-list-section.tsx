import Loading from "@/components/loading";
import type { Provider } from "@/lib/api";
import { ProvidersDesktopTable } from "./providers-desktop-table";
import { ProvidersMobileList } from "./providers-mobile-list";

interface ProvidersListSectionProps {
  loading: boolean;
  hasFilter: boolean;
  providers: Provider[];
  updatingFilter: Record<number, boolean>;
  updatingAssociationTrigger: Record<number, boolean>;
  clearingAssociation: boolean;

  onOpenAllModelsDialog: (provider: Provider) => void | Promise<void>;
  onToggleModelEndpoint: (provider: Provider) => void | Promise<void>;
  onToggleAssociationTrigger: (provider: Provider, checked: boolean) => void | Promise<void>;
  onToggleModelFilter: (provider: Provider, checked: boolean) => void | Promise<void>;
  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onOpenClearAssociationsDialog: (providerId: number) => void;
  onCancelClearAssociationsDialog: () => void;
  onHandleClearAssociations: () => void | Promise<void>;

  onOpenDeleteDialog: (providerId: number) => void;
  onCancelDeleteDialog: () => void;
  onHandleDelete: () => void | Promise<void>;
}

export function ProvidersListSection({
  loading,
  hasFilter,
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  clearingAssociation,
  onOpenAllModelsDialog,
  onToggleModelEndpoint,
  onToggleAssociationTrigger,
  onToggleModelFilter,
  onEditProvider,
  onOpenModelsDialog,
  onOpenClearAssociationsDialog,
  onCancelClearAssociationsDialog,
  onHandleClearAssociations,
  onOpenDeleteDialog,
  onCancelDeleteDialog,
  onHandleDelete,
}: ProvidersListSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-xl bg-background shadow-sm">
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载提供商列表" />
        </div>
      ) : providers.length === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm text-center px-6">
          {hasFilter ? "未找到匹配的提供商" : "暂无提供商数据"}
        </div>
      ) : (
        <div className="h-full flex flex-col">
          <ProvidersDesktopTable
            providers={providers}
            updatingFilter={updatingFilter}
            updatingAssociationTrigger={updatingAssociationTrigger}
            clearingAssociation={clearingAssociation}
            onOpenAllModelsDialog={onOpenAllModelsDialog}
            onToggleModelEndpoint={onToggleModelEndpoint}
            onToggleAssociationTrigger={onToggleAssociationTrigger}
            onToggleModelFilter={onToggleModelFilter}
            onEditProvider={onEditProvider}
            onOpenModelsDialog={onOpenModelsDialog}
            onOpenClearAssociationsDialog={onOpenClearAssociationsDialog}
            onCancelClearAssociationsDialog={onCancelClearAssociationsDialog}
            onHandleClearAssociations={onHandleClearAssociations}
            onOpenDeleteDialog={onOpenDeleteDialog}
            onCancelDeleteDialog={onCancelDeleteDialog}
            onHandleDelete={onHandleDelete}
          />
          <ProvidersMobileList
            providers={providers}
            updatingFilter={updatingFilter}
            updatingAssociationTrigger={updatingAssociationTrigger}
            clearingAssociation={clearingAssociation}
            onOpenAllModelsDialog={onOpenAllModelsDialog}
            onToggleModelEndpoint={onToggleModelEndpoint}
            onToggleAssociationTrigger={onToggleAssociationTrigger}
            onToggleModelFilter={onToggleModelFilter}
            onEditProvider={onEditProvider}
            onOpenModelsDialog={onOpenModelsDialog}
            onOpenClearAssociationsDialog={onOpenClearAssociationsDialog}
            onCancelClearAssociationsDialog={onCancelClearAssociationsDialog}
            onHandleClearAssociations={onHandleClearAssociations}
            onOpenDeleteDialog={onOpenDeleteDialog}
            onCancelDeleteDialog={onCancelDeleteDialog}
            onHandleDelete={onHandleDelete}
          />
        </div>
      )}
    </div>
  );
}

