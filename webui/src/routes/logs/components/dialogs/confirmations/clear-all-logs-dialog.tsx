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

type ClearAllLogsDialogProps = {
  open: boolean;
  clearing: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function ClearAllLogsDialog({
  open,
  clearing,
  onOpenChange,
  onConfirm,
}: ClearAllLogsDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认清空日志</AlertDialogTitle>
          <AlertDialogDescription>
            确定要清空所有请求日志和对话记录吗？此操作不可恢复。
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
