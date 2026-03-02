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

type ClearHealthCheckLogsDialogProps = {
  open: boolean;
  clearing: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function ClearHealthCheckLogsDialog({
  open,
  clearing,
  onOpenChange,
  onConfirm,
}: ClearHealthCheckLogsDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认清空检测日志</AlertDialogTitle>
          <AlertDialogDescription>
            删除所有健康检测日志，操作不可恢复。确认继续？
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={clearing}>取消</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            disabled={clearing}
          >
            {clearing ? "清空中..." : "确认清空"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
