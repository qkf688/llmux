import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { ModelSyncLog } from "@/lib/api";
import { MinusCircle, XCircle } from "lucide-react";
import { StatusBadge } from "../shared/status-badge";
import { parseModelSyncError } from "../../utils/error-parser";
import { formatSyncDate } from "../../utils/formatters";

type ModelSyncDetailDialogProps = {
  log: ModelSyncLog | null;
  onClose: () => void;
};

export function ModelSyncDetailDialog({ log, onClose }: ModelSyncDetailDialogProps) {
  return (
    <Dialog open={log !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>同步详情</DialogTitle>
          <DialogDescription>
            {log?.ProviderName} - {log ? formatSyncDate(log.SyncedAt) : "-"}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 overflow-y-auto flex-1">
          {log && (
            <div className="flex items-center gap-2 p-3 bg-muted rounded-md">
              <span className="text-sm text-muted-foreground">状态:</span>
              <StatusBadge status={log.Status} />
            </div>
          )}

          {log && log.Status === "error" && log.Error && (
            <ErrorInfoSection errorMessage={log.Error} />
          )}

          {log && log.AddedCount > 0 && (
            <div>
              <h3 className="font-semibold text-green-600 mb-2">新增模型 ({log.AddedCount})</h3>
              <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
                {(log.AddedModels ?? []).map((model, index) => (
                  <div key={`${model}-${index}`} className="text-sm">
                    {model}
                  </div>
                ))}
              </div>
            </div>
          )}

          {log && log.RemovedCount > 0 && (
            <div>
              <h3 className="font-semibold text-red-600 mb-2">删除模型 ({log.RemovedCount})</h3>
              <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
                {(log.RemovedModels ?? []).map((model, index) => (
                  <div key={`${model}-${index}`} className="text-sm">
                    {model}
                  </div>
                ))}
              </div>
            </div>
          )}

          {log && log.Status === "unchanged" && (
            <div className="p-3 bg-blue-50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-900 rounded-md">
              <p className="text-sm text-blue-700 dark:text-blue-400 flex items-center gap-2">
                <MinusCircle className="h-4 w-4" />
                此次同步未检测到模型变化
              </p>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

type ErrorInfoSectionProps = {
  errorMessage: string;
};

function ErrorInfoSection({ errorMessage }: ErrorInfoSectionProps) {
  const parsedError = parseModelSyncError(errorMessage);

  return (
    <div className="p-3 bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-900 rounded-md">
      <h3 className="font-semibold text-red-600 mb-2 flex items-center gap-2">
        <XCircle className="h-4 w-4" />
        错误信息
      </h3>

      {parsedError.statusCode && (
        <div className="mb-2">
          <span className="text-sm font-medium text-red-700 dark:text-red-400">
            HTTP状态码: <span className="font-mono">{parsedError.statusCode}</span>
          </span>
        </div>
      )}

      {parsedError.responseBody && (
        <div className="mb-2">
          <span className="text-sm font-medium text-red-700 dark:text-red-400 block mb-1">响应内容:</span>
          <pre className="text-xs text-red-700 dark:text-red-400 bg-red-100 dark:bg-red-950/40 p-2 rounded overflow-x-auto">
            {parsedError.responseBody}
          </pre>
        </div>
      )}

      {!parsedError.statusCode && !parsedError.responseBody && (
        <div className="text-sm text-red-700 dark:text-red-400 whitespace-pre-wrap break-words">
          {parsedError.originalError}
        </div>
      )}
    </div>
  );
}
