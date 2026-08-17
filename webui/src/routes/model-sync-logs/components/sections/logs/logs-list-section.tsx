import type { ReactNode } from "react";
import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { ModelSyncLog } from "@/lib/api";
import { LogsMobileList } from "./logs-mobile-list";
import { LogsTableDesktop } from "./logs-table-desktop";

type LogsListSectionProps = {
  loading: boolean;
  logs: ModelSyncLog[];
  allSelected: boolean;
  /** 分页器：渲染在卡片内底部 */
  footer?: ReactNode;
  isLogSelected: (id: number) => boolean;
  onToggleSelectAll: () => void;
  onToggleSelectLog: (id: number, checked: boolean) => void;
  onOpenDetail: (log: ModelSyncLog) => void;
};

export function LogsListSection({
  loading,
  logs,
  allSelected,
  footer,
  isLogSelected,
  onToggleSelectAll,
  onToggleSelectLog,
  onOpenDetail,
}: LogsListSectionProps) {
  return (
    <TableCard footer={footer}>
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载日志" />
        </div>
      ) : logs.length === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm">暂无同步日志</div>
      ) : (
        <div className="h-full overflow-auto">
          <LogsTableDesktop
            logs={logs}
            allSelected={allSelected}
            isLogSelected={isLogSelected}
            onToggleSelectAll={onToggleSelectAll}
            onToggleSelectLog={onToggleSelectLog}
            onOpenDetail={onOpenDetail}
          />
          <LogsMobileList
            logs={logs}
            isLogSelected={isLogSelected}
            onToggleSelectLog={onToggleSelectLog}
            onOpenDetail={onOpenDetail}
          />
        </div>
      )}
    </TableCard>
  );
}
