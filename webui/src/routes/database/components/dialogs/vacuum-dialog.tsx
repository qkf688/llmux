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
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  vacuuming: boolean;
};

export function VacuumDialog({ open, onOpenChange, onConfirm, vacuuming }: VacuumDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认执行 VACUUM 操作</AlertDialogTitle>
          <AlertDialogDescription>
            这将压缩数据库并回收空间，可能需要一些时间。在操作期间，数据库将被锁定，无法进行其他操作。确定要继续吗？
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={vacuuming}>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={vacuuming}>
            {vacuuming ? "压缩中..." : "确认压缩"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
