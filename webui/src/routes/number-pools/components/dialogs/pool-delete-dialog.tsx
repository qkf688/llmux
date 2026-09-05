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

type PoolDeleteDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 待删除号池名（表单用值由 hook 持有，弹窗只读展示） */
  poolName: string;
  onConfirm: () => void;
};

/**
 * 号池删除确认弹窗。后端有引用守卫：被分组引用时返回 400，
 * 描述中提前提示，避免用户重复尝试。
 */
export function PoolDeleteDialog({
  open,
  onOpenChange,
  poolName,
  onConfirm,
}: PoolDeleteDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确定要删除号池「{poolName}」吗？</AlertDialogTitle>
          <AlertDialogDescription>
            此操作无法撤销。将永久删除该号池及其下全部凭据；被分组引用时无法删除。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          {/* 点击后 Radix 立即关闭弹窗，loading 反馈交给 toast，防重复提交由 hook 内 guard 负责 */}
          <AlertDialogAction onClick={onConfirm}>确认删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}