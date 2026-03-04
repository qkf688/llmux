import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ChatLog } from "@/lib/api";
import { Trash2 } from "lucide-react";
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
    <div className="flex-1 overflow-y-auto hidden sm:block w-full">
      <Table className="min-w-[850px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
          <TableRow className="hover:bg-secondary/90">
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
            <TableHead className="w-[180px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => (
            <TableRow key={log.ID} className={selectedIds.has(log.ID) ? "bg-muted/50" : ""}>
              <TableCell>
                <Checkbox
                  checked={selectedIds.has(log.ID)}
                  onCheckedChange={(checked) => onSelectOne(log.ID, checked === true)}
                  aria-label={`选择日志 ${log.ID}`}
                />
              </TableCell>
              <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                {formatDateTime(log.CreatedAt)}
              </TableCell>
              <TableCell className="font-medium">
                <div className="flex items-center gap-2">
                  <span>{log.Name}</span>
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
                    log.Status === "success" ? "text-green-500" : "text-red-500"
                  }`}
                >
                  {log.Status}
                </span>
              </TableCell>
              <TableCell>{formatTime(log.ChunkTime)}</TableCell>
              <TableCell className="max-w-[120px] truncate text-xs" title={log.ProviderModel}>
                {log.ProviderModel}
              </TableCell>
               <TableCell className="text-xs">
                 {log.has_format_conversion ? (
                   <Badge variant="outline" className="text-xs">
                     {log.source_format} → {log.target_format}
                   </Badge>
                 ) : (
                  <Badge variant="outline" className="text-xs">
                    {log.Style || "-"}
                  </Badge>
                 )}
               </TableCell>
              <TableCell className="text-xs">{log.ProviderName}</TableCell>
              <TableCell>
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
                    onClick={() => onOpenDelete(log.ID)}
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
