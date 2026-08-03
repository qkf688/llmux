import { Button } from "@/components/ui/button";
import { Eye, Loader2 } from "lucide-react";

type HealthCheckBannerProps = {
  batchId: string | null;
  resultDialogOpen: boolean;
  completed: boolean;
  onShowProgress: () => void;
  onDismiss: () => void;
};

export function HealthCheckBanner({
  batchId,
  resultDialogOpen,
  completed,
  onShowProgress,
  onDismiss,
}: HealthCheckBannerProps) {
  if (!batchId || resultDialogOpen) {
    return null;
  }

  if (completed) {
    return (
      <div className="flex-shrink-0 rounded-lg border bg-success-tint border-success/30 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-success">健康检测已完成</span>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={onShowProgress} className="h-7">
              <Eye className="size-4 mr-1" />
              查看结果
            </Button>
            <Button variant="ghost" size="sm" onClick={onDismiss} className="h-7 text-muted-foreground">
              关闭
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-shrink-0 rounded-lg border bg-primary/10 border-primary/30 px-4 py-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Loader2 className="size-4 animate-spin text-primary" />
          <span>健康检测正在后台运行中...</span>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onShowProgress} className="h-7">
            <Eye className="size-4 mr-1" />
            查看进度
          </Button>
          <Button variant="ghost" size="sm" onClick={onDismiss} className="h-7 text-muted-foreground">
            忽略
          </Button>
        </div>
      </div>
    </div>
  );
}
