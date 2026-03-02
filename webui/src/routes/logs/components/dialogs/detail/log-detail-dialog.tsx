import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { ChatLog } from "@/lib/api";
import {
  formatDateTime,
  formatDurationValue,
  formatTokenValue,
  formatTpsValue,
} from "../../../utils/formatters";
import { DetailCard } from "./detail-card";
import { RequestResponseSection } from "./request-response-section";

type LogDetailDialogProps = {
  open: boolean;
  log: ChatLog | null;
  onOpenChange: (open: boolean) => void;
  onExportRequestResponse: (log: ChatLog) => void;
};

export function LogDetailDialog({ open, log, onOpenChange, onExportRequestResponse }: LogDetailDialogProps) {
  if (!log) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-auto sm:max-w-2xl max-h-[85vh] sm:max-h-[90vh] flex flex-col p-4">
        <div className="p-3 border-b flex-shrink-0 sm:p-4">
          <DialogHeader className="p-0">
            <DialogTitle>日志详情: {log.ID}</DialogTitle>
          </DialogHeader>
        </div>

        <div className="overflow-y-auto p-3 flex-1">
          <div className="space-y-4 text-sm">
            <div className="space-y-2">
              <div className="text-sm">
                <span className="text-muted-foreground">创建时间：</span>
                <span>{formatDateTime(log.CreatedAt)}</span>
              </div>
              <div className="text-sm">
                <span className="text-muted-foreground">状态：</span>
                <span className={log.Status === "success" ? "text-green-600" : "text-red-600"}>
                  {log.Status}
                </span>
              </div>
            </div>

            {log.Error && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 p-2 sm:p-3">
                <p className="text-xs text-destructive uppercase tracking-wide">错误信息</p>
                <div className="text-destructive whitespace-pre-wrap break-words text-sm">{log.Error}</div>
              </div>
            )}

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">基本信息</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
                <DetailCard label="模型名称" value={log.Name} />
                <DetailCard label="模型类型" value={log.is_virtual_model ? "虚拟模型" : "真实模型"} />
                <DetailCard label="提供商" value={log.ProviderName || "-"} />
                <DetailCard label="提供商模型" value={log.ProviderModel || "-"} mono />
                <DetailCard label="客户端类型" value={log.Style || "-"} />
                <DetailCard
                  label="格式转换"
                  value={
                    log.has_format_conversion
                      ? `${log.source_format} → ${log.target_format}`
                      : "未发生转换"
                  }
                />
                <DetailCard label="用户代理" value={log.UserAgent || "-"} mono />
                <DetailCard label="远端 IP" value={log.RemoteIP || "-"} mono />
                <DetailCard label="记录 IO" value={log.ChatIO ? "是" : "否"} />
                <DetailCard label="重试次数" value={log.Retry ?? 0} />
              </div>
            </div>

            <RequestResponseSection log={log} onExportRequestResponse={onExportRequestResponse} />

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">性能指标</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
                <DetailCard label="代理耗时" value={formatDurationValue(log.ProxyTime)} />
                <DetailCard label="首包耗时" value={formatDurationValue(log.FirstChunkTime)} />
                <DetailCard label="完成耗时" value={formatDurationValue(log.ChunkTime)} />
                <DetailCard label="TPS" value={formatTpsValue(log.Tps)} />
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Token 使用</p>
              <div className="grid grid-cols-1 sm:grid-cols-4 gap-3 sm:gap-4">
                <DetailCard label="输入" value={formatTokenValue(log.prompt_tokens)} />
                <DetailCard label="输出" value={formatTokenValue(log.completion_tokens)} />
                <DetailCard label="总计" value={formatTokenValue(log.total_tokens)} />
                <DetailCard
                  label="缓存"
                  value={formatTokenValue(log.prompt_tokens_details?.cached_tokens)}
                />
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
