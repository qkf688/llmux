import { useEffect, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { getLogDetail, type ChatLog } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import {
  formatDateTime,
  formatDurationValue,
  formatTokenValue,
  formatTpsValue,
} from "../../../utils/formatters";
import { DetailCard } from "./detail-card";
import { RequestResponseSection } from "./request-response-section";
import { TransformDiffSection } from "./transform-diff-section";
import type { ChatLogExportSections } from "../../../utils/export-log";

type LogDetailDialogProps = {
  open: boolean;
  log: ChatLog | null;
  onOpenChange: (open: boolean) => void;
  onExportLog: (log: ChatLog, sections: ChatLogExportSections) => void | Promise<void>;
};

const needsLogDetail = (log: ChatLog) =>
  log.RequestHeaders === undefined &&
  log.RequestBody === undefined &&
  log.RawRequestBody === undefined &&
  log.ResponseHeaders === undefined &&
  log.ResponseBody === undefined &&
  log.RawResponseBody === undefined;

export function LogDetailDialog({ open, log, onOpenChange, onExportLog }: LogDetailDialogProps) {
  const [detailLog, setDetailLog] = useState<ChatLog | null>(log);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  useEffect(() => {
    setDetailLog(log);
    setDetailError(null);
    setDetailLoading(false);
  }, [log]);

  useEffect(() => {
    if (!open || !log) {
      return;
    }
    if (!needsLogDetail(log)) {
      return;
    }

    let cancelled = false;
    setDetailLoading(true);
    void (async () => {
      try {
        const full = await getLogDetail(log.ID);
        if (!cancelled) {
          setDetailLog(full);
        }
      } catch (error) {
        if (!cancelled) {
          setDetailError(toErrorMessage(error));
        }
      } finally {
        if (!cancelled) {
          setDetailLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [open, log]);

  if (!log || !detailLog) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-auto sm:max-w-2xl max-h-[85vh] sm:max-h-[90vh] flex flex-col p-4">
        <div className="p-3 border-b flex-shrink-0 sm:p-4">
          <DialogHeader className="p-0">
            <DialogTitle>日志详情: {detailLog.ID}</DialogTitle>
            <DialogDescription>查看请求/响应内容、性能指标与错误信息</DialogDescription>
          </DialogHeader>
        </div>

        <div className="overflow-y-auto p-3 flex-1">
          <div className="space-y-4 text-sm">
            <div className="space-y-2">
              <div className="text-sm">
                <span className="text-muted-foreground">创建时间：</span>
                <span>{formatDateTime(detailLog.CreatedAt)}</span>
              </div>
              <div className="text-sm">
                <span className="text-muted-foreground">状态：</span>
                <span className={detailLog.Status === "success" ? "text-green-600" : "text-red-600"}>
                  {detailLog.Status}
                </span>
              </div>
            </div>

            {detailLog.Error && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 p-2 sm:p-3">
                <p className="text-xs text-destructive uppercase tracking-wide">错误信息</p>
                <div className="text-destructive whitespace-pre-wrap break-words text-sm">{detailLog.Error}</div>
              </div>
            )}

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">基本信息</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
                <DetailCard label="模型名称" value={detailLog.Name} />
                <DetailCard label="模型类型" value={detailLog.is_virtual_model ? "虚拟模型" : "真实模型"} />
                <DetailCard label="提供商" value={detailLog.ProviderName || "-"} />
                <DetailCard label="提供商模型" value={detailLog.ProviderModel || "-"} mono />
                <DetailCard label="客户端类型" value={detailLog.Style || "-"} />
                  <DetailCard
                   label="格式"
                   value={
                      detailLog.has_format_conversion
                        ? `${detailLog.source_format} → ${detailLog.target_format}`
                        : "未发生转换"
                    }
                  />
                <DetailCard label="用户代理" value={detailLog.UserAgent || "-"} mono />
                <DetailCard label="远端 IP" value={detailLog.RemoteIP || "-"} mono />
                <DetailCard label="记录 IO" value={detailLog.ChatIO ? "是" : "否"} />
                <DetailCard label="重试次数" value={detailLog.Retry ?? 0} />
              </div>
            </div>

            {detailError && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 p-2 sm:p-3">
                <p className="text-xs text-destructive uppercase tracking-wide">加载请求响应失败</p>
                <div className="text-destructive whitespace-pre-wrap break-words text-sm">{detailError}</div>
              </div>
            )}

            <RequestResponseSection
              log={detailLog}
              loading={detailLoading}
              onExportLog={onExportLog}
            />

            <TransformDiffSection log={detailLog} />

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">性能指标</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
                <DetailCard label="代理耗时" value={formatDurationValue(detailLog.ProxyTime)} />
                <DetailCard label="首包耗时" value={formatDurationValue(detailLog.FirstChunkTime)} />
                <DetailCard label="完成耗时" value={formatDurationValue(detailLog.ChunkTime)} />
                <DetailCard label="TPS" value={formatTpsValue(detailLog.Tps)} />
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Token 使用</p>
              <div className="grid grid-cols-1 sm:grid-cols-4 gap-3 sm:gap-4">
                <DetailCard label="输入" value={formatTokenValue(detailLog.prompt_tokens)} />
                <DetailCard label="输出" value={formatTokenValue(detailLog.completion_tokens)} />
                <DetailCard label="总计" value={formatTokenValue(detailLog.total_tokens)} />
                <DetailCard
                  label="缓存"
                  value={formatTokenValue(detailLog.prompt_tokens_details?.cached_tokens)}
                />
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
