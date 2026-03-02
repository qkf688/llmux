import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

interface BatchDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedCount: number;
  deleting: boolean;
  onConfirm: () => void;
}

export function BatchDeleteDialog({
  open,
  onOpenChange,
  selectedCount,
  deleting,
  onConfirm,
}: BatchDeleteDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogTrigger asChild>
        <Button variant="destructive" className="w-full sm:w-auto">
          批量删除 ({selectedCount})
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确定要批量删除这些模型吗？</AlertDialogTitle>
          <AlertDialogDescription>
            此操作无法撤销。这将永久删除选中的 {selectedCount} 个模型。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={deleting}>
            {deleting ? "删除中..." : "确认删除"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
