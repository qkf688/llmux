import Loading from "@/components/loading";
import type { HealthCheckLog } from "@/lib/api";
import { HealthCheckLogsDesktopTable } from "./health-check-logs-desktop-table";
import { HealthCheckLogsMobileList } from "./health-check-logs-mobile-list";

type HealthCheckLogsListSectionProps = {
  loading: boolean;
  hasLogs: boolean;
  logs: HealthCheckLog[];
  onOpenDetail: (log: HealthCheckLog) => void;
};

export function HealthCheckLogsListSection({
  loading,
  hasLogs,
  logs,
  onOpenDetail,
}: HealthCheckLogsListSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载健康检测日志" />
        </div>
      ) : !hasLogs ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">暂无健康检测日志</div>
      ) : (
        <div className="h-full flex flex-col">
          <HealthCheckLogsDesktopTable logs={logs} onOpenDetail={onOpenDetail} />
          <HealthCheckLogsMobileList logs={logs} onOpenDetail={onOpenDetail} />
        </div>
      )}
    </div>
  );
}
