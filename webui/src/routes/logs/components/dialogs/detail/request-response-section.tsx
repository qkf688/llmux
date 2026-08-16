import Loading from "@/components/loading";
import { Button } from "@/components/ui/button";
import type { ChatLog } from "@/lib/api";
import { Download } from "lucide-react";
import { useState } from "react";
import { formatByteLength } from "../../../utils/formatters";
import { DEFAULT_CHAT_LOG_EXPORT_SECTIONS, type ChatLogExportSectionKey, type ChatLogExportSections } from "../../../utils/export-log";
import { ExportLogDialog } from "./export-log-dialog";

type RequestResponseSectionProps = {
  log: ChatLog;
  loading?: boolean;
  onExportLog: (log: ChatLog, sections: ChatLogExportSections) => void | Promise<void>;
};

export function RequestResponseSection({
  log,
  loading,
  onExportLog,
}: RequestResponseSectionProps) {
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [sections, setSections] = useState<ChatLogExportSections>(DEFAULT_CHAT_LOG_EXPORT_SECTIONS);

  if (loading) {
    return (
      <div className="rounded-md border bg-muted/20 p-3">
        <Loading message="加载请求响应内容" />
      </div>
    );
  }

  const hasRequestResponseContent = Boolean(
    log.request_headers || log.request_body || log.raw_request_body || log.response_headers || log.raw_response_body || log.response_body
  );

  if (!hasRequestResponseContent) {
    return null;
  }

  const openExportDialog = () => {
    setSections(DEFAULT_CHAT_LOG_EXPORT_SECTIONS);
    setExportDialogOpen(true);
  };

  const toggleSection = (key: ChatLogExportSectionKey) => {
    setSections((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const confirmExport = () => {
    void (async () => {
      try {
        setExporting(true);
        await onExportLog(log, sections);
        setExportDialogOpen(false);
      } catch {
        // toast handled by caller
      } finally {
        setExporting(false);
      }
    })();
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">请求响应内容</p>
        <Button variant="outline" size="sm" onClick={openExportDialog}>
          <Download className="size-4 mr-1" />
          导出
        </Button>
      </div>

      <div className="space-y-3">
        {log.request_headers && (
          <div className="rounded-md border bg-muted/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">
              请求头 ({formatByteLength(log.request_headers)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              {log.request_headers}
            </pre>
          </div>
        )}

        {log.raw_request_body && (
          <div className="rounded-md border bg-info-tint p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-info-foreground uppercase tracking-wide">
              {log.request_body ? "原始请求体 - 转换前" : "请求体"} ({formatByteLength(log.raw_request_body)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-info-foreground">
              {log.raw_request_body}
            </pre>
          </div>
        )}

        {log.request_body && (
          <div className="rounded-md border bg-success-tint p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-success-foreground uppercase tracking-wide">
              {log.raw_request_body ? "请求体 - 转换后" : "请求体"} ({formatByteLength(log.request_body)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-success-foreground">
              {log.request_body}
            </pre>
          </div>
        )}

        {log.response_headers && (
          <div className="rounded-md border bg-muted/20 p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">
              响应头 ({formatByteLength(log.response_headers)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              {log.response_headers}
            </pre>
          </div>
        )}

        {log.raw_response_body && (
          <div className="rounded-md border bg-info-tint p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-info-foreground uppercase tracking-wide">
              {log.response_body ? "原始响应体 - 转换前" : "响应体"} ({formatByteLength(log.raw_response_body)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-info-foreground">
              {log.raw_response_body}
            </pre>
          </div>
        )}

        {log.response_body && (
          <div className="rounded-md border bg-success-tint p-2 space-y-1 sm:p-3">
            <p className="text-[11px] text-success-foreground uppercase tracking-wide">
              响应体 - 转换后 ({formatByteLength(log.response_body)})
            </p>
            <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-success-foreground">
              {log.response_body}
            </pre>
          </div>
        )}
      </div>

      <ExportLogDialog
        open={exportDialogOpen}
        onOpenChange={setExportDialogOpen}
        exporting={exporting}
        sections={sections}
        onToggleSection={toggleSection}
        onConfirm={confirmExport}
      />
    </div>
  );
}
