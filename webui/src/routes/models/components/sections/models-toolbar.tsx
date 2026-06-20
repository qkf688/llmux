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
    <div className="flex flex-col gap-3 flex-shrink-0">
      {/* 第一行：标题 + 搜索框 */}
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold tracking-tight whitespace-nowrap">模型管理</h2>
        <Input
          placeholder="搜索模型名称..."
          value={searchQuery}
          onChange={(event) => onSearchQueryChange(event.target.value)}
          className="flex-1 max-w-sm"
        />
      </div>

      {/* 第二行：操作按钮（靠右） */}
      <div className="flex flex-wrap items-center justify-start gap-2">
        <Button
          onClick={onOpenCreateDialog}
          className="bg-green-600 hover:bg-green-700 text-white"
        >
          添加模型
        </Button>
        {selectedCount > 0 && (
          <>
            <Button
              variant="default"
              onClick={onOpenBatchSettings}
              className="relative bg-green-600 hover:bg-green-700 text-white"
            >
              批量设置
              <span className="absolute -top-1.5 -right-1.5 inline-flex items-center justify-center min-w-4 h-4 px-1 rounded-full text-[10px] font-bold leading-none bg-red-500 text-white">
                {selectedCount}
              </span>
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
      </div>
    </div>
  );
}
