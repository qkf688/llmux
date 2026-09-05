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

/** 待删除凭据目标：单条（带 id + 掩码）或批量（只带条数） */
export type CredentialDeleteTarget =
  | { kind: "single"; id: number; keyMasked: string }
  | { kind: "bulk"; count: number };

type CredentialDeleteDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** null 表示关闭态；弹窗只读展示目标信息，删除动作由调用方在 onConfirm 触发 */
  target: CredentialDeleteTarget | null;
  onConfirm: () => void;
};

/**
 * 凭据删除确认弹窗。删除不可撤销，单条展示 key 掩码、批量展示条数；
 * 与号池删除弹窗保持同一 AlertDialog 交互形态。
 */
export function CredentialDeleteDialog({
  open,
  onOpenChange,
  target,
  onConfirm,
}: CredentialDeleteDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {target?.kind === "single"
              ? "确定要删除这条凭据吗？"
              : target
                ? `确定要删除选中的 ${target.count} 条凭据吗？`
                : ""}
          </AlertDialogTitle>
          <AlertDialogDescription>
            此操作无法撤销。
            {target?.kind === "single" ? (
              <>
                将永久删除凭据{" "}
                <span className="font-mono text-xs text-foreground">{target.keyMasked}</span>
                ，其失败次数与使用记录一并清除。
              </>
            ) : target ? (
              <>将永久删除选中的 {target.count} 条凭据及其使用记录。</>
            ) : null}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          {/* 点击后 Radix 立即关闭弹窗，loading/toast 反馈由调用方 handleDelete 负责 */}
          <AlertDialogAction onClick={onConfirm}>确认删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
