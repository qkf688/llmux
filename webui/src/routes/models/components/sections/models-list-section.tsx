import Loading from "@/components/loading";
import type { Model } from "@/lib/api";
import { ModelsDesktopTable } from "./models-desktop-table";
import { ModelsMobileList } from "./models-mobile-list";

interface ModelsListSectionProps {
  loading: boolean;
  totalCount: number;
  models: Model[];
  selectedIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  togglingIOLog: Record<number, boolean>;
  togglingAutoAssociate: Record<number, boolean>;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onToggleIOLog: (model: Model) => void;
  onToggleAutoAssociate: (model: Model, checked: boolean) => void;
  onAssociate: (model: Model) => void;
  onEdit: (model: Model) => void;
  onDelete: (model: Model) => void;
}

export function ModelsListSection({
  loading,
  totalCount,
  models,
  selectedIds,
  isAllSelected,
  isPartialSelected,
  togglingIOLog,
  togglingAutoAssociate,
  onSelectAll,
  onSelectOne,
  onToggleIOLog,
  onToggleAutoAssociate,
  onAssociate,
  onEdit,
  onDelete,
}: ModelsListSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载模型列表" />
        </div>
      ) : totalCount === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">暂无模型数据</div>
      ) : models.length === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">没有找到匹配的模型</div>
      ) : (
        <div className="h-full flex flex-col">
          <ModelsDesktopTable
            models={models}
            selectedIds={selectedIds}
            isAllSelected={isAllSelected}
            isPartialSelected={isPartialSelected}
            togglingIOLog={togglingIOLog}
            togglingAutoAssociate={togglingAutoAssociate}
            onSelectAll={onSelectAll}
            onSelectOne={onSelectOne}
            onToggleIOLog={onToggleIOLog}
            onToggleAutoAssociate={onToggleAutoAssociate}
            onAssociate={onAssociate}
            onEdit={onEdit}
            onDelete={onDelete}
          />
          <ModelsMobileList
            models={models}
            selectedIds={selectedIds}
            isAllSelected={isAllSelected}
            isPartialSelected={isPartialSelected}
            togglingIOLog={togglingIOLog}
            onSelectAll={onSelectAll}
            onSelectOne={onSelectOne}
            onToggleIOLog={onToggleIOLog}
            onAssociate={onAssociate}
            onEdit={onEdit}
            onDelete={onDelete}
          />
        </div>
      )}
    </div>
  );
}
