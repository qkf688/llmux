import type { AssociationBatchTestResult } from "../../types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { CheckCircle, TestTube, TestTubes, Trash2, X, XCircle } from "lucide-react";

type BatchActionSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedAssociationCount: number;
  filteredAssociationCount: number;
  batchUpdatingStatus: boolean;
  batchTesting: boolean;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  onBatchUpdateStatus: (status: boolean) => void;
  onBatchTestSelected: () => void;
  onBatchTestAll: () => void;
  onSelectAllSuccessful: () => void;
  onSelectAllFailed: () => void;
  onOpenBatchDeleteDialog: () => void;
};

export function BatchActionSheet({
  open,
  onOpenChange,
  selectedAssociationCount,
  filteredAssociationCount,
  batchUpdatingStatus,
  batchTesting,
  associationTestResults,
  onBatchUpdateStatus,
  onBatchTestSelected,
  onBatchTestAll,
  onSelectAllSuccessful,
  onSelectAllFailed,
  onOpenBatchDeleteDialog,
}: BatchActionSheetProps) {
  const successfulCount = Object.values(associationTestResults).filter((result) => result.success === true).length;
  const failedCount = Object.values(associationTestResults).filter((result) => result.success === false).length;
  const hasSuccess = successfulCount > 0;
  const hasFailed = failedCount > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg p-0 gap-0 [&>button]:hidden">
        <div className="flex flex-col max-h-[80vh]">
          <DialogHeader className="px-4 py-3 border-b">
            <div className="flex items-center justify-between">
              <DialogTitle className="text-base">批量操作</DialogTitle>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => onOpenChange(false)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            <DialogDescription className="text-xs">
              对选中的关联执行批量启用/停用、测试、选择与删除操作
            </DialogDescription>
          </DialogHeader>

          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground uppercase tracking-wide px-1">状态操作</p>
              <div className="grid grid-cols-2 gap-2">
                <Button
                  variant="outline"
                  disabled={selectedAssociationCount === 0 || batchUpdatingStatus}
                  onClick={() => {
                    onBatchUpdateStatus(true);
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  {batchUpdatingStatus ? (
                    <Spinner className="h-5 w-5" />
                  ) : (
                    <CheckCircle className="h-5 w-5 text-green-600" />
                  )}
                  <span className="text-xs font-medium">批量启用</span>
                  {selectedAssociationCount > 0 && (
                    <span className="text-[10px] text-muted-foreground">{selectedAssociationCount} 项</span>
                  )}
                </Button>
                <Button
                  variant="outline"
                  disabled={selectedAssociationCount === 0 || batchUpdatingStatus}
                  onClick={() => {
                    onBatchUpdateStatus(false);
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  {batchUpdatingStatus ? (
                    <Spinner className="h-5 w-5" />
                  ) : (
                    <XCircle className="h-5 w-5 text-orange-600" />
                  )}
                  <span className="text-xs font-medium">批量停用</span>
                  {selectedAssociationCount > 0 && (
                    <span className="text-[10px] text-muted-foreground">{selectedAssociationCount} 项</span>
                  )}
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-xs text-muted-foreground uppercase tracking-wide px-1">测试操作</p>
              <div className="grid grid-cols-2 gap-2">
                <Button
                  variant="outline"
                  disabled={selectedAssociationCount === 0 || batchTesting}
                  onClick={() => {
                    onBatchTestSelected();
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  {batchTesting ? <Spinner className="h-5 w-5" /> : <TestTube className="h-5 w-5" />}
                  <span className="text-xs font-medium">测试选中</span>
                  {selectedAssociationCount > 0 && (
                    <span className="text-[10px] text-muted-foreground">{selectedAssociationCount} 项</span>
                  )}
                </Button>
                <Button
                  variant="outline"
                  disabled={filteredAssociationCount === 0 || batchTesting}
                  onClick={() => {
                    onBatchTestAll();
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  {batchTesting ? <Spinner className="h-5 w-5" /> : <TestTubes className="h-5 w-5" />}
                  <span className="text-xs font-medium">测试全部</span>
                  <span className="text-[10px] text-muted-foreground">{filteredAssociationCount} 项</span>
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-xs text-muted-foreground uppercase tracking-wide px-1">选择操作</p>
              <div className="grid grid-cols-2 gap-2">
                <Button
                  variant="outline"
                  disabled={!hasSuccess}
                  onClick={() => {
                    onSelectAllSuccessful();
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  <CheckCircle className="h-5 w-5 text-green-600" />
                  <span className="text-xs font-medium">选择成功</span>
                  {successfulCount > 0 && <span className="text-[10px] text-muted-foreground">{successfulCount} 项</span>}
                </Button>
                <Button
                  variant="outline"
                  disabled={!hasFailed}
                  onClick={() => {
                    onSelectAllFailed();
                    onOpenChange(false);
                  }}
                  className="h-16 flex flex-col gap-1 items-center justify-center"
                >
                  <XCircle className="h-5 w-5 text-red-600" />
                  <span className="text-xs font-medium">选择失败</span>
                  {failedCount > 0 && <span className="text-[10px] text-muted-foreground">{failedCount} 项</span>}
                </Button>
              </div>
            </div>

            <div className="space-y-2 pt-2 border-t">
              <Button
                variant="destructive"
                disabled={selectedAssociationCount === 0}
                onClick={() => {
                  onOpenBatchDeleteDialog();
                  onOpenChange(false);
                }}
                className="w-full h-12 flex items-center justify-center gap-2"
              >
                <Trash2 className="h-5 w-5" />
                <span className="font-medium">批量删除</span>
                {selectedAssociationCount > 0 && <span className="ml-auto">{selectedAssociationCount} 项</span>}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

