import { Boxes } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { BatchDeleteDialog } from "../dialogs/batch-delete-dialog";

interface ModelsToolbarProps {
  /** 模型总数（未筛选），用于页头副标题 */
  totalCount: number;
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
  totalCount,
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
    <>
      <PageHeader
        icon={Boxes}
        title="模型管理"
        subtitle={`共 ${totalCount} 个真实模型`}
        actions={
          <>
            <Button size="sm" onClick={onOpenCreateDialog}>
              添加模型
            </Button>
            {selectedCount > 0 && (
              <>
                <Button
                  variant="default"
                  size="sm"
                  onClick={onOpenBatchSettings}
                  className="relative"
                >
                  批量设置
                  <span className="absolute -top-1.5 -right-1.5 inline-flex items-center justify-center min-w-4 h-4 px-1 rounded-full text-[10px] font-bold leading-none bg-destructive text-destructive-foreground">
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
          </>
        }
      />

      {/* 搜索独立卡片：与 providers 页一致，页头只放标题栏 */}
      <div className="flex flex-wrap items-center gap-2 rounded-xl border bg-card px-2.5 py-2 shadow-sm flex-shrink-0">
        <Input
          placeholder="搜索模型名称..."
          value={searchQuery}
          onChange={(event) => onSearchQueryChange(event.target.value)}
          className="flex-1 max-w-sm"
        />
      </div>
    </>
  );
}
