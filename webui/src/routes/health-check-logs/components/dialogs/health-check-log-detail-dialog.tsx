import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { HealthCheckLog } from "@/lib/api";
import { formatDateTime, formatResponseTime, isHealthCheckSuccess } from "../../utils/formatters";
import { DetailCard } from "../shared/detail-card";

type HealthCheckLogDetailDialogProps = {
  open: boolean;
  log: HealthCheckLog | null;
  onOpenChange: (open: boolean) => void;
};

export function HealthCheckLogDetailDialog({
  open,
  log,
  onOpenChange,
}: HealthCheckLogDetailDialogProps) {
  if (!log) {
    return null;
  }

  const success = isHealthCheckSuccess(log.status);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="p-0 w-[92vw] sm:w-auto sm:max-w-2xl max-h-[95vh] flex flex-col">
        <div className="p-4 border-b flex-shrink-0">
          <DialogHeader className="p-0">
            <DialogTitle>检测详情: {log.ID}</DialogTitle>
          </DialogHeader>
        </div>

        <div className="overflow-y-auto p-3 flex-1">
          <div className="space-y-6 text-sm">
            <div className="space-y-3">
              <div className="space-y-2">
                <div className="text-sm">
                  <span className="text-muted-foreground">检测时间：</span>
                  <span>{formatDateTime(log.checked_at)}</span>
                </div>
                <div className="text-sm">
                  <span className="text-muted-foreground">状态：</span>
                  <span className={success ? "text-green-600" : "text-red-600"}>
                    {success ? "成功" : "失败"}
                  </span>
                </div>
              </div>
            </div>

            {log.error && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3">
                <p className="text-xs text-destructive uppercase tracking-wide mb-1">错误信息</p>
                <div className="text-destructive whitespace-pre-wrap break-words text-sm">{log.error}</div>
              </div>
            )}

            <div className="space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">基本信息</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <DetailCard label="模型名称" value={log.model_name} />
                <DetailCard label="提供商" value={log.provider_name || "-"} />
                <DetailCard label="提供商模型" value={log.provider_model || "-"} mono />
                <DetailCard label="响应时间" value={formatResponseTime(log.response_time)} />
                <DetailCard label="模型提供商ID" value={log.model_provider_id} />
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
