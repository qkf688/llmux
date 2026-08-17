import type { ReactNode } from "react";
import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { ChatLog } from "@/lib/api";
import { LogsDesktopTable } from "./logs-desktop-table";
import { LogsMobileList } from "./logs-mobile-list";

type LogsListSectionProps = {
  loading: boolean;
  hasLogs: boolean;
  logs: ChatLog[];
  /** 当前页码：透传到 mobile 列表用于翻页触发 stagger 重播 */
  page: number;
  selectedIds: Set<number>;
  isAllSelected: boolean;
  isSomeSelected: boolean;
  deleting: boolean;
  /** 分页器：渲染在卡片内底部 */
  footer?: ReactNode;
  canViewChatIO: (log: ChatLog) => boolean;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onOpenDetail: (log: ChatLog) => void;
  onViewChatIO: (log: ChatLog) => void;
  onOpenDelete: (id: number) => void;
};

export function LogsListSection({
  loading,
  hasLogs,
  logs,
  page,
  selectedIds,
  isAllSelected,
  isSomeSelected,
  deleting,
  footer,
  canViewChatIO,
  onSelectAll,
  onSelectOne,
  onOpenDetail,
  onViewChatIO,
  onOpenDelete,
}: LogsListSectionProps) {
  return (
    <TableCard footer={footer}>
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载日志数据" />
        </div>
      ) : !hasLogs ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">暂无请求日志</div>
      ) : (
        <div className="h-full flex flex-col">
          <LogsDesktopTable
            logs={logs}
            selectedIds={selectedIds}
            isAllSelected={isAllSelected}
            isSomeSelected={isSomeSelected}
            deleting={deleting}
            canViewChatIO={canViewChatIO}
            onSelectAll={onSelectAll}
            onSelectOne={onSelectOne}
            onOpenDetail={onOpenDetail}
            onViewChatIO={onViewChatIO}
            onOpenDelete={onOpenDelete}
          />
          <LogsMobileList
            logs={logs}
            page={page}
            selectedIds={selectedIds}
            deleting={deleting}
            canViewChatIO={canViewChatIO}
            onSelectOne={onSelectOne}
            onOpenDetail={onOpenDetail}
            onViewChatIO={onViewChatIO}
            onOpenDelete={onOpenDelete}
          />
        </div>
      )}
    </TableCard>
  );
}
