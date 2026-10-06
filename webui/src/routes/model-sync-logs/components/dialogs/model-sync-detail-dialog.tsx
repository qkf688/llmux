import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { badgeVariants } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { copyTextWithToast } from "@/lib/clipboard";
import type { ModelSyncLog } from "@/lib/api";
import { MinusCircle, XCircle } from "lucide-react";
import { StatusBadge } from "../shared/status-badge";
import { parseModelSyncError } from "../../utils/error-parser";
import { formatDateTime } from "@/lib/formatters";

type ModelSyncDetailDialogProps = {
  log: ModelSyncLog | null;
  onClose: () => void;
};

export function ModelSyncDetailDialog({ log, onClose }: ModelSyncDetailDialogProps) {
  return (
    <Dialog open={log !== null} onOpenChange={(open) => !open && onClose()}>
      {/* md 而非 lg：同步结果是打开后只读的静态快照，内容量从挂载到卸载是常量。
          lg/xl 的固定高度是留给「生命周期内内容会变」（切 tab、流式追加）的，
          用在这里只会让短内容底部留出一大片空白。 */}
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>同步详情</DialogTitle>
          <DialogDescription>
            {log?.ProviderName} - {log ? formatDateTime(log.SyncedAt) : "-"}
          </DialogDescription>
        </DialogHeader>

        <DialogBody className="space-y-4">
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
              <h3 className="font-semibold text-success mb-2">新增模型 ({log.AddedCount})</h3>
              <ModelChipList models={log.AddedModels ?? []} />
            </div>
          )}

          {log && log.RemovedCount > 0 && (
            <div>
              <h3 className="font-semibold text-destructive mb-2">删除模型 ({log.RemovedCount})</h3>
              <ModelChipList models={log.RemovedModels ?? []} />
            </div>
          )}

          {log && log.Status === "unchanged" && (
            <div className="p-3 bg-info-tint border border-info/20 rounded-md">
              <p className="text-sm text-info-foreground flex items-center gap-2">
                <MinusCircle className="h-4 w-4" />
                此次同步未检测到模型变化
              </p>
            </div>
          )}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

type ModelChipListProps = {
  models: string[];
};

// 模型名以 chip 流呈现而不是逐行文本：模型标识符长短悬殊（`big-pickle` 到
// `nvidia/nemotron-3-ultra-550b-a55b:free`），逐行排布会让 40+ 项变成一堵纵向文本墙，
// chip 自动换行把 45 行压到十余行，视线能横着扫过。
//
// chip 不用完整描边：gap 只有 6px，1px 实线描边挨着挨着会连成网格噪声并压过文字，
// 边界改由浅底色 + inset ring 承担。模型名按标识符用等宽字体，与「最近新增模型」表格一致。
//
// 每个 chip 是 button（不是 Badge 的 div）：点击热区 + 键盘可达。复制走
// copyTextWithToast —— 三级降级提示由 lib/clipboard 单源承载，成功文案带上模型名本身
// （沿用凭据那条「提示必须带内容标识」的纪律）。模型名非机密，故不用 copySecretWithToast。
function ModelChipList({ models }: ModelChipListProps) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {models.map((model, index) => (
        <button
          key={`${model}-${index}`}
          type="button"
          onClick={() => void copyTextWithToast(model, `已复制模型名: ${model}`)}
          className={cn(
            badgeVariants({ variant: "outline" }),
            "cursor-pointer border-transparent bg-muted/60 ring-1 ring-inset ring-border/40 font-mono text-[11px] font-normal px-1.5 hover:bg-muted",
          )}
        >
          {model}
        </button>
      ))}
    </div>
  );
}

type ErrorInfoSectionProps = {
  errorMessage: string;
};

function ErrorInfoSection({ errorMessage }: ErrorInfoSectionProps) {
  const parsedError = parseModelSyncError(errorMessage);

  return (
    <div className="p-3 bg-destructive-tint border border-destructive/20 rounded-md">
      <h3 className="font-semibold text-destructive mb-2 flex items-center gap-2">
        <XCircle className="h-4 w-4" />
        错误信息
      </h3>

      {parsedError.statusCode && (
        <div className="mb-2">
          <span className="text-sm font-medium text-destructive-tint-foreground">
            HTTP状态码: <span className="font-mono">{parsedError.statusCode}</span>
          </span>
        </div>
      )}

      {parsedError.responseBody && (
        <div className="mb-2">
          <span className="text-sm font-medium text-destructive-tint-foreground block mb-1">响应内容:</span>
          <pre className="text-xs text-destructive-tint-foreground bg-destructive/10 p-2 rounded overflow-x-auto">
            {parsedError.responseBody}
          </pre>
        </div>
      )}

      {!parsedError.statusCode && !parsedError.responseBody && (
        <div className="text-sm text-destructive-tint-foreground whitespace-pre-wrap break-words">
          {parsedError.originalError}
        </div>
      )}
    </div>
  );
}
