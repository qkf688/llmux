import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { ModelSyncLog } from "@/lib/api";
import { StatusBadge } from "../../shared/status-badge";
import { formatSyncDate } from "../../../utils/formatters";

type LogsTableDesktopProps = {
  logs: ModelSyncLog[];
  allSelected: boolean;
  isLogSelected: (id: number) => boolean;
  onToggleSelectAll: () => void;
  onToggleSelectLog: (id: number, checked: boolean) => void;
  onOpenDetail: (log: ModelSyncLog) => void;
};

export function LogsTableDesktop({
  logs,
  allSelected,
  isLogSelected,
  onToggleSelectAll,
  onToggleSelectLog,
  onOpenDetail,
}: LogsTableDesktopProps) {
  return (
    <div className="hidden sm:block w-full">
      <Table>
        <TableHeader className="sticky top-0 bg-secondary/80">
          <TableRow>
            <TableHead className="w-12">
              <Checkbox checked={allSelected} onCheckedChange={() => onToggleSelectAll()} />
            </TableHead>
            <TableHead>提供商</TableHead>
            <TableHead>同步时间</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>变化</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => (
            <TableRow key={log.ID}>
              <TableCell>
                <Checkbox
                  checked={isLogSelected(log.ID)}
                  onCheckedChange={(checked) => onToggleSelectLog(log.ID, checked === true)}
                />
              </TableCell>
              <TableCell className="font-medium">{log.ProviderName}</TableCell>
              <TableCell className="text-sm text-muted-foreground">{formatSyncDate(log.SyncedAt)}</TableCell>
              <TableCell>
                <StatusBadge status={log.Status} />
              </TableCell>
              <TableCell>
                <div className="flex gap-2">
                  {log.AddedCount > 0 && <span className="text-green-600">+{log.AddedCount}</span>}
                  {log.RemovedCount > 0 && <span className="text-red-600">-{log.RemovedCount}</span>}
                  {log.AddedCount === 0 && log.RemovedCount === 0 && log.Status !== "error" && (
                    <span className="text-muted-foreground text-sm">-</span>
                  )}
                </div>
              </TableCell>
              <TableCell>
                <Button variant="outline" size="sm" onClick={() => onOpenDetail(log)}>
                  查看详情
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
