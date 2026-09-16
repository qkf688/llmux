import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  STICKY_ACTIONS_CELL_CLS,
  STICKY_ACTIONS_HEAD_CLS,
  STICKY_HEADER_CLS,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type { ChatLog } from "@/lib/api";
import { Trash2 } from "lucide-react";
import { LogScheduleCell } from "../../shared";
import { formatDateTime, formatTime } from "../../../utils/formatters";

type LogsDesktopTableProps = {
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

export function LogsDesktopTable({
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
}: LogsDesktopTableProps) {
  return (
    <div className="flex-1 overflow-auto hidden sm:block w-full">
      <Table className="min-w-[850px]">
        <TableHeader className={`z-10 ${STICKY_HEADER_CLS}`}>
          <TableRow>
            <TableHead className="w-[40px]">
              <Checkbox
                checked={isSomeSelected ? "indeterminate" : isAllSelected}
                onCheckedChange={(checked) => onSelectAll(checked === true)}
                aria-label="全选"
              />
            </TableHead>
            <TableHead>时间</TableHead>
            <TableHead>模型名称</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>耗时</TableHead>
            <TableHead>提供商模型</TableHead>
            <TableHead>格式</TableHead>
            <TableHead>提供商</TableHead>
            <TableHead>密钥</TableHead>
            <TableHead>调度</TableHead>
            {/* 操作列钉右：横向滚动时始终可见，与 providers / 关联表格一致 */}
            <TableHead className={`w-[180px] ${STICKY_ACTIONS_HEAD_CLS}`}>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => (
            <TableRow key={log.id} className={cn("group", selectedIds.has(log.id) && "bg-muted/50")}>
              <TableCell>
                <Checkbox
                  checked={selectedIds.has(log.id)}
                  onCheckedChange={(checked) => onSelectOne(log.id, checked === true)}
                  aria-label={`选择日志 ${log.id}`}
                />
              </TableCell>
              <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                {formatDateTime(log.created_at)}
              </TableCell>
              <TableCell className="font-medium">
                <div className="flex items-center gap-2">
                  <span>{log.name}</span>
                  {log.is_virtual_model && (
                    <Badge variant="secondary" className="text-xs">
                      虚拟
                    </Badge>
                  )}
                </div>
              </TableCell>
              <TableCell>
                <span
                  className={`inline-flex items-center px-2 py-1 ${
                    log.status === "success" ? "text-success" : "text-destructive"
                  }`}
                >
                  {log.status}
                </span>
              </TableCell>
              <TableCell>{formatTime(log.chunk_time)}</TableCell>
              <TableCell className="max-w-[120px] truncate text-xs" title={log.provider_model}>
                {log.provider_model}
              </TableCell>
               <TableCell className="text-xs">
                 {log.has_format_conversion ? (
                   <Badge variant="outline" className="text-xs">
                     {log.source_format} → {log.target_format}
                   </Badge>
                 ) : (
                  <Badge variant="outline" className="text-xs">
                    {log.style || "-"}
                  </Badge>
                 )}
               </TableCell>
              <TableCell className="text-xs">{log.provider_name}</TableCell>
              <TableCell className="max-w-[120px] truncate text-xs" title={log.api_key_name}>
                {log.api_key_name || "-"}
              </TableCell>
              <TableCell>
                <LogScheduleCell log={log} />
              </TableCell>
              <TableCell
                className={cn(
                  STICKY_ACTIONS_CELL_CLS,
                  selectedIds.has(log.id) && "bg-[var(--table-head-bg)]",
                )}
              >
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" className="h-8 px-2" onClick={() => onOpenDetail(log)}>
                    详情
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2"
                    onClick={() => onViewChatIO(log)}
                    disabled={!canViewChatIO(log)}
                  >
                    会话
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2 text-destructive hover:text-destructive"
                    onClick={() => onOpenDelete(log.id)}
                    disabled={deleting}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
