import { Button } from "@/components/ui/button";
import { Eye, Play, RefreshCw, Trash2 } from "lucide-react";

type HealthCheckHeaderProps = {
  runningBackgroundCheck: boolean;
  clearingLogs: boolean;
  onRunHealthCheck: () => void;
  onOpenClearDialog: () => void;
  onRefresh: () => void;
};

export function HealthCheckHeader({
  runningBackgroundCheck,
  clearingLogs,
  onRunHealthCheck,
  onOpenClearDialog,
  onRefresh,
}: HealthCheckHeaderProps) {
  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="text-2xl font-bold tracking-tight">健康检测日志</h2>
          <p className="text-sm text-muted-foreground">查看模型提供商的健康检测历史记录</p>
        </div>

        <div className="flex gap-2 ml-auto">
          <Button onClick={onRunHealthCheck} variant="default" className="shrink-0">
            {runningBackgroundCheck ? (
              <>
                <Eye className="size-4 mr-2" />
                查看进度
              </>
            ) : (
              <>
                <Play className="size-4 mr-2" />
                执行检测
              </>
            )}
          </Button>

          <Button
            variant="destructive"
            className="shrink-0"
            disabled={clearingLogs}
            onClick={onOpenClearDialog}
          >
            <Trash2 className="size-4 mr-2" />
            {clearingLogs ? "清空中..." : "清空检测日志"}
          </Button>

          <Button
            onClick={onRefresh}
            variant="outline"
            size="icon"
            className="shrink-0"
            aria-label="刷新列表"
            title="刷新列表"
          >
            <RefreshCw className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
