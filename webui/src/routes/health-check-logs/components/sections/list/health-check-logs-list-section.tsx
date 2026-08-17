import type { ReactNode } from "react";
import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { HealthCheckLog } from "@/lib/api";
import { HealthCheckLogsDesktopTable } from "./health-check-logs-desktop-table";
import { HealthCheckLogsMobileList } from "./health-check-logs-mobile-list";

type HealthCheckLogsListSectionProps = {
  loading: boolean;
  hasLogs: boolean;
  logs: HealthCheckLog[];
  /** 分页器：渲染在卡片内底部 */
  footer?: ReactNode;
  onOpenDetail: (log: HealthCheckLog) => void;
};

export function HealthCheckLogsListSection({
  loading,
  hasLogs,
  logs,
  footer,
  onOpenDetail,
}: HealthCheckLogsListSectionProps) {
  return (
    <TableCard footer={footer}>
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
    </TableCard>
  );
}
