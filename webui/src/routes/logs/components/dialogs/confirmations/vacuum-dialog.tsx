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

type VacuumDialogProps = {
  open: boolean;
  vacuuming: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function VacuumDialog({ open, vacuuming, onOpenChange, onConfirm }: VacuumDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>执行 VACUUM</AlertDialogTitle>
          <AlertDialogDescription>
            将对数据库执行 VACUUM 以回收空间，过程会短暂锁库，建议在低峰期操作。是否继续？
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={vacuuming}>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={vacuuming}>
            {vacuuming ? "执行中..." : "确认执行"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
