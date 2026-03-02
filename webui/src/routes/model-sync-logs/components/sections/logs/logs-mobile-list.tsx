import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import type { ModelSyncLog } from "@/lib/api";
import { StatusBadge } from "../../shared/status-badge";
import { formatSyncDate } from "../../../utils/formatters";

type LogsMobileListProps = {
  logs: ModelSyncLog[];
  isLogSelected: (id: number) => boolean;
  onToggleSelectLog: (id: number, checked: boolean) => void;
  onOpenDetail: (log: ModelSyncLog) => void;
};

export function LogsMobileList({
  logs,
  isLogSelected,
  onToggleSelectLog,
  onOpenDetail,
}: LogsMobileListProps) {
  return (
    <div className="sm:hidden px-2 py-3 divide-y divide-border">
      {logs.map((log) => (
        <div
          key={log.ID}
          className={`py-3 space-y-2 my-1 px-1 ${isLogSelected(log.ID) ? "bg-muted/50 rounded" : ""}`}
        >
          <div className="flex items-start justify-between gap-2">
            <div className="flex items-start gap-2 min-w-0 flex-1">
              <Checkbox
                checked={isLogSelected(log.ID)}
                onCheckedChange={(checked) => onToggleSelectLog(log.ID, checked === true)}
                className="mt-1"
              />
              <div className="min-w-0 flex-1">
                <h3 className="font-semibold text-sm truncate">{log.ProviderName}</h3>
                <p className="text-[11px] text-muted-foreground">{formatSyncDate(log.SyncedAt)}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <StatusBadge status={log.Status} textClassName="text-xs" iconClassName="h-3.5 w-3.5" />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 text-xs ml-6">
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">新增</p>
              <p className={`font-medium ${log.AddedCount > 0 ? "text-green-600" : "text-muted-foreground"}`}>
                {log.AddedCount > 0 ? `+${log.AddedCount}` : "-"}
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">删除</p>
              <p className={`font-medium ${log.RemovedCount > 0 ? "text-red-600" : "text-muted-foreground"}`}>
                {log.RemovedCount > 0 ? `-${log.RemovedCount}` : "-"}
              </p>
            </div>
          </div>

          <div className="flex justify-end ml-6">
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => onOpenDetail(log)}>
              查看详情
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
