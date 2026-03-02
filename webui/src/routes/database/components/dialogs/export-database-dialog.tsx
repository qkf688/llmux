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

type ExportDatabaseDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  exporting: boolean;
};

export function ExportDatabaseDialog({
  open,
  onOpenChange,
  onConfirm,
  exporting,
}: ExportDatabaseDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>导出完整数据库文件</AlertDialogTitle>
          <AlertDialogDescription>
            <div className="space-y-2">
              <p>⚠️ 警告：数据库文件包含所有敏感信息，包括：</p>
              <ul className="list-disc list-inside space-y-1 ml-2">
                <li>API 密钥和访问令牌</li>
                <li>提供商配置信息</li>
                <li>模型配置和关联关系</li>
                <li>系统设置和日志</li>
              </ul>
              <p className="font-medium">请妥善保管导出的文件，避免泄露敏感信息。</p>
              <p className="text-sm text-muted-foreground">
                文件格式：SQLite 数据库 (.db)，可用于完整备份和恢复。
              </p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={exporting}>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={exporting}>
            {exporting ? "导出中..." : "确认导出"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
