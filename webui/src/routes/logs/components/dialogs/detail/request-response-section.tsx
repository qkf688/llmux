import Loading from "@/components/loading";
import { Button } from "@/components/ui/button";
import type { ChatLog } from "@/lib/api";
import { Download } from "lucide-react";
import { useState } from "react";
import { DEFAULT_CHAT_LOG_EXPORT_SECTIONS, type ChatLogExportSectionKey, type ChatLogExportSections } from "../../../utils/export-log";
import { BodyTextBlock } from "./body-text-block";
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
          <BodyTextBlock label="请求头" text={log.request_headers} />
        )}

        {log.raw_request_body && (
          <BodyTextBlock
            label={log.request_body ? "原始请求体 - 转换前" : "请求体"}
            text={log.raw_request_body}
            tone="info"
          />
        )}

        {log.request_body && (
          <BodyTextBlock
            label={log.raw_request_body ? "请求体 - 转换后" : "请求体"}
            text={log.request_body}
            tone="success"
          />
        )}

        {log.response_headers && (
          <BodyTextBlock label="响应头" text={log.response_headers} />
        )}

        {log.raw_response_body && (
          <BodyTextBlock
            label={log.response_body ? "原始响应体 - 转换前" : "响应体"}
            text={log.raw_response_body}
            tone="info"
          />
        )}

        {log.response_body && (
          <BodyTextBlock label="响应体 - 转换后" text={log.response_body} tone="success" />
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
