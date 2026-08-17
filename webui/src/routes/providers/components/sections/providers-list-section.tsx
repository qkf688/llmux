import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { Provider } from "@/lib/api";
import { ProvidersDesktopTable } from "./providers-desktop-table";
import { ProvidersMobileList } from "./providers-mobile-list";

interface ProvidersListSectionProps {
  loading: boolean;
  hasFilter: boolean;
  providers: Provider[];
  updatingFilter: Record<number, boolean>;
  updatingAssociationTrigger: Record<number, boolean>;

  onOpenAllModelsDialog: (provider: Provider) => void | Promise<void>;
  onToggleModelEndpoint: (provider: Provider) => void | Promise<void>;
  onToggleAssociationTrigger: (provider: Provider, checked: boolean) => void | Promise<void>;
  onToggleModelFilter: (provider: Provider, checked: boolean) => void | Promise<void>;
  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onHandleClearAssociations: (providerId: number) => void | Promise<void>;
  onHandleDelete: (providerId: number) => void | Promise<void>;
}

export function ProvidersListSection({
  loading,
  hasFilter,
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  onOpenAllModelsDialog,
  onToggleModelEndpoint,
  onToggleAssociationTrigger,
  onToggleModelFilter,
  onEditProvider,
  onOpenModelsDialog,
  onHandleClearAssociations,
  onHandleDelete,
}: ProvidersListSectionProps) {
  return (
    <TableCard>
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
            onOpenAllModelsDialog={onOpenAllModelsDialog}
            onToggleModelEndpoint={onToggleModelEndpoint}
            onToggleAssociationTrigger={onToggleAssociationTrigger}
            onToggleModelFilter={onToggleModelFilter}
            onEditProvider={onEditProvider}
            onOpenModelsDialog={onOpenModelsDialog}
            onHandleClearAssociations={onHandleClearAssociations}
            onHandleDelete={onHandleDelete}
          />
          <ProvidersMobileList
            providers={providers}
            updatingFilter={updatingFilter}
            updatingAssociationTrigger={updatingAssociationTrigger}
            onOpenAllModelsDialog={onOpenAllModelsDialog}
            onToggleModelEndpoint={onToggleModelEndpoint}
            onToggleAssociationTrigger={onToggleAssociationTrigger}
            onToggleModelFilter={onToggleModelFilter}
            onEditProvider={onEditProvider}
            onOpenModelsDialog={onOpenModelsDialog}
            onHandleClearAssociations={onHandleClearAssociations}
            onHandleDelete={onHandleDelete}
          />
        </div>
      )}
    </TableCard>
  );
}

