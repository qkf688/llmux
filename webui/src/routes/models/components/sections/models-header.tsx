import { Boxes } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { BatchDeleteDialog } from "../dialogs/batch-delete-dialog";

interface ModelsHeaderProps {
  /** 模型总数（未筛选），用于页头副标题 */
  totalCount: number;
  selectedCount: number;
  batchDeleteDialogOpen: boolean;
  onBatchDeleteDialogOpenChange: (open: boolean) => void;
  batchDeleting: boolean;
  onConfirmBatchDelete: () => void;
  onOpenCreateDialog: () => void;
}

/**
 * models 页头：标题栏 + 新建 / 批量操作按钮（选中时才出现）。
 * 搜索框职责在 ModelsToolbar（SRP）。
 */
export function ModelsHeader({
  totalCount,
  selectedCount,
  batchDeleteDialogOpen,
  onBatchDeleteDialogOpenChange,
  batchDeleting,
  onConfirmBatchDelete,
  onOpenCreateDialog,
}: ModelsHeaderProps) {
  return (
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
            <BatchDeleteDialog
              open={batchDeleteDialogOpen}
              onOpenChange={onBatchDeleteDialogOpenChange}
              selectedCount={selectedCount}
              deleting={batchDeleting}
              onConfirm={onConfirmBatchDelete}
            />
          )}
        </>
      }
    />
  );
}
