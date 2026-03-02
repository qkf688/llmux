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

interface ModelDeleteDialogProps {
  open: boolean;
  modelLabel: string | number;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function ModelDeleteDialog({
  open,
  modelLabel,
  onOpenChange,
  onConfirm,
}: ModelDeleteDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确定要删除这个模型吗？</AlertDialogTitle>
          <AlertDialogDescription>
            此操作无法撤销。这将永久删除模型 {modelLabel}。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>确认删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
