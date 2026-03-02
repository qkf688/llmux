import Loading from "@/components/loading";
import type { ChatLog } from "@/lib/api";
import { LogsDesktopTable } from "./logs-desktop-table";
import { LogsMobileList } from "./logs-mobile-list";

type LogsListSectionProps = {
  loading: boolean;
  hasLogs: boolean;
  logs: ChatLog[];
  selectedIds: Set<number>;
  isAllSelected: boolean;
  isSomeSelected: boolean;
  deleting: boolean;
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
  selectedIds,
  isAllSelected,
  isSomeSelected,
  deleting,
  canViewChatIO,
  onSelectAll,
  onSelectOne,
  onOpenDetail,
  onViewChatIO,
  onOpenDelete,
}: LogsListSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
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
    </div>
  );
}
