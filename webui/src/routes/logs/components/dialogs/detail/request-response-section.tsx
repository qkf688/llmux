import { Button } from "@/components/ui/button";
import type { ChatLog } from "@/lib/api";
import { Download } from "lucide-react";
import { formatByteLength } from "../../../utils/formatters";

type RequestResponseSectionProps = {
  log: ChatLog;
  onExportRequestResponse: (log: ChatLog) => void;
};

export function RequestResponseSection({ log, onExportRequestResponse }: RequestResponseSectionProps) {
  const hasRequestResponseContent = Boolean(
    log.RequestHeaders || log.RequestBody || log.ResponseHeaders || log.RawResponseBody || log.ResponseBody
  );

  if (!hasRequestResponseContent) {
    return null;
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">请求响应内容</p>
        <Button variant="outline" size="sm" onClick={() => onExportRequestResponse(log)}>
          <Download className="size-4 mr-1" />
          导出请求响应
        </Button>
      </div>

      <div className="space-y-3">
        {log.RequestHeaders && (
          <div className="rounded-md border bg-muted/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">
              请求头 ({formatByteLength(log.RequestHeaders)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              {log.RequestHeaders}
            </pre>
          </div>
        )}

        {log.RequestBody && (
          <div className="rounded-md border bg-muted/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">
              请求体 ({formatByteLength(log.RequestBody)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              {log.RequestBody}
            </pre>
          </div>
        )}

        {log.ResponseHeaders && (
          <div className="rounded-md border bg-muted/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">
              响应头 ({formatByteLength(log.ResponseHeaders)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              {log.ResponseHeaders}
            </pre>
          </div>
        )}

        {log.RawResponseBody && (
          <div className="rounded-md border bg-blue-50 dark:bg-blue-950/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-blue-700 dark:text-blue-400 uppercase tracking-wide">
              {log.ResponseBody ? "原始响应体 - 转换前" : "响应体"} ({formatByteLength(log.RawResponseBody)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-blue-900 dark:text-blue-200">
              {log.RawResponseBody}
            </pre>
          </div>
        )}

        {log.ResponseBody && (
          <div className="rounded-md border bg-green-50 dark:bg-green-950/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-green-700 dark:text-green-400 uppercase tracking-wide">
              响应体 - 转换后 ({formatByteLength(log.ResponseBody)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-green-900 dark:text-green-200">
              {log.ResponseBody}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}
