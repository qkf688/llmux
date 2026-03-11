import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

type ClearFilteredLogsDialogProps = {
  open: boolean;
  clearing: boolean;
  filtersSummary?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function ClearFilteredLogsDialog({
  open,
  clearing,
  filtersSummary,
  onOpenChange,
  onConfirm,
}: ClearFilteredLogsDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认清空筛选结果</AlertDialogTitle>
          <AlertDialogDescription>
            <div className="flex flex-col gap-2">
              <div>确定要清空当前筛选结果吗？此操作会跨分页删除所有匹配的请求日志与对话记录，且不可恢复。</div>
              {filtersSummary && (
                <div className="text-xs text-muted-foreground">
                  <span className="font-medium text-foreground">当前筛选：</span>
                  {filtersSummary}
                </div>
              )}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={clearing}>取消</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={clearing}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {clearing ? "清空中..." : "确认清空"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

