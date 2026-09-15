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

type ApiKeyDeleteDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 待删除 key 名（目标由 page hook 持有，弹窗只读展示） */
  keyName: string;
  onConfirm: () => void;
};

/**
 * API Key 删除确认。文案交代两个既有语义（方案 §2/§4），避免用户以为会连带清历史：
 * 硬删立即生效；历史日志的密钥归因是写入时快照，不受删除影响；在途请求不受影响。
 */
export function ApiKeyDeleteDialog({
  open,
  onOpenChange,
  keyName,
  onConfirm,
}: ApiKeyDeleteDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确定要删除 API Key「{keyName}」吗？</AlertDialogTitle>
          <AlertDialogDescription>
            删除立即生效且不可撤销：持此 key 的调用将无法访问 /v1（进行中的请求不受影响）。
            历史请求日志中的密钥归因是写入时快照，不会因删除而丢失。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          {/* 点击后 Radix 立即关闭弹窗，loading 反馈交给 toast（号池同款先例） */}
          <AlertDialogAction onClick={onConfirm}>确认删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
