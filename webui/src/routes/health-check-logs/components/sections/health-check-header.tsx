import { Activity, Eye, Play, RefreshCw, Trash2 } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

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
    <PageHeader
      icon={Activity}
      title="健康检测日志"
      subtitle="查看模型提供商的健康检测历史记录"
      actions={
        <>
          <Button onClick={onRunHealthCheck} variant="default" size="sm" className="shrink-0">
            {runningBackgroundCheck ? (
              <>
                <Eye className="size-4" />
                查看进度
              </>
            ) : (
              <>
                <Play className="size-4" />
                执行检测
              </>
            )}
          </Button>

          <Button
            variant="destructive"
            size="sm"
            className="shrink-0"
            disabled={clearingLogs}
            onClick={onOpenClearDialog}
          >
            <Trash2 className="size-4" />
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
        </>
      }
    />
  );
}
