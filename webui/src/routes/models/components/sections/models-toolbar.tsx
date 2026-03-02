import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { BatchDeleteDialog } from "../dialogs/batch-delete-dialog";

interface ModelsToolbarProps {
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  selectedCount: number;
  batchDeleteDialogOpen: boolean;
  onBatchDeleteDialogOpenChange: (open: boolean) => void;
  batchDeleting: boolean;
  onOpenBatchSettings: () => void;
  onConfirmBatchDelete: () => void;
  onOpenCreateDialog: () => void;
}

export function ModelsToolbar({
  searchQuery,
  onSearchQueryChange,
  selectedCount,
  batchDeleteDialogOpen,
  onBatchDeleteDialogOpenChange,
  batchDeleting,
  onOpenBatchSettings,
  onConfirmBatchDelete,
  onOpenCreateDialog,
}: ModelsToolbarProps) {
  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-3">
          <h2 className="text-2xl font-bold tracking-tight">模型管理</h2>
          <Input
            placeholder="搜索模型名称..."
            value={searchQuery}
            onChange={(event) => onSearchQueryChange(event.target.value)}
            className="w-48"
          />
        </div>
        <div className="flex w-full sm:w-auto items-center justify-end gap-2">
          {selectedCount > 0 && (
            <>
              <Button variant="default" className="w-full sm:w-auto" onClick={onOpenBatchSettings}>
                批量设置 ({selectedCount})
              </Button>
              <BatchDeleteDialog
                open={batchDeleteDialogOpen}
                onOpenChange={onBatchDeleteDialogOpenChange}
                selectedCount={selectedCount}
                deleting={batchDeleting}
                onConfirm={onConfirmBatchDelete}
              />
            </>
          )}
          <Button onClick={onOpenCreateDialog} className="w-full sm:w-auto sm:min-w-[120px]">
            添加模型
          </Button>
        </div>
      </div>
    </div>
  );
}
