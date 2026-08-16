import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import type { ChatLog } from "@/lib/api";
import { Trash2 } from "lucide-react";
import { formatByteLength, formatDateTime, formatTime } from "../../../utils/formatters";

type LogsMobileListProps = {
  logs: ChatLog[];
  /** 当前页码，作为 stagger 重播的触发标识 */
  page: number;
  selectedIds: Set<number>;
  deleting: boolean;
  canViewChatIO: (log: ChatLog) => boolean;
  onSelectOne: (id: number, checked: boolean) => void;
  onOpenDetail: (log: ChatLog) => void;
  onViewChatIO: (log: ChatLog) => void;
  onOpenDelete: (id: number) => void;
};

export function LogsMobileList({
  logs,
  page,
  selectedIds,
  deleting,
  canViewChatIO,
  onSelectOne,
  onOpenDetail,
  onViewChatIO,
  onOpenDelete,
}: LogsMobileListProps) {
  return (
    // resetKey 用页码而非数据内容：翻页时整批错峰重现，页内单条删除不会重挂载容器
    //（本元素同时是滚动容器，重挂载会把滚动位置重置到顶部）
    <StaggerList
      className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border"
      resetKey={page}
    >
      {logs.map((log) => (
        <StaggerItem
          key={log.id}
          className={`py-2 space-y-2 my-0.5 px-1 ${selectedIds.has(log.id) ? "bg-muted/50 rounded" : ""}`}
        >
          <div className="flex items-start gap-2 min-w-0">
            <div className="flex items-start gap-2 min-w-0 flex-1">
              <Checkbox
                checked={selectedIds.has(log.id)}
                onCheckedChange={(checked) => onSelectOne(log.id, checked === true)}
                className="mt-0.5 shrink-0"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-start gap-2 min-w-0">
                  <h3 className="font-semibold text-[13px] leading-snug break-words overflow-hidden [display:-webkit-box] [-webkit-line-clamp:2] [-webkit-box-orient:vertical]">
                    {log.name}
                  </h3>
                  {log.is_virtual_model && (
                    <Badge variant="secondary" className="shrink-0 text-[10px] px-1.5 py-0.5">
                      虚拟
                    </Badge>
                  )}
                </div>
                <div className="flex items-center gap-2 pt-0.5">
                  <p className="min-w-0 text-[10px] text-muted-foreground leading-tight truncate">
                    {formatDateTime(log.created_at)}
                  </p>
                  <span
                    className={`shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded-full ${
                      log.status === "success"
                        ? "bg-success-tint text-success-foreground"
                        : "bg-destructive-tint text-destructive-tint-foreground"
                    }`}
                  >
                    {log.status}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-x-3 gap-y-2 text-[11px] ml-6">
            <div className="space-y-0.5">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">耗时</p>
              <p className="font-medium">{formatTime(log.chunk_time)}</p>
            </div>
            <div className="space-y-0.5">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">提供商</p>
              <p className="truncate">{log.provider_name}</p>
            </div>
            <div className="space-y-0.5">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">格式</p>
               <p>
                 {log.has_format_conversion ? (
                   <Badge variant="outline" className="text-[10px] px-1.5 py-0.5">
                     {log.source_format} → {log.target_format}
                   </Badge>
                 ) : (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0.5">
                    {log.style || "-"}
                  </Badge>
                 )}
               </p>
             </div>
            <div className="space-y-0.5">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">请求头</p>
              <p className="font-medium">{formatByteLength(log.request_headers)}</p>
            </div>
            <div className="space-y-0.5 col-span-2">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">响应头</p>
              <p className="font-medium">{formatByteLength(log.response_headers)}</p>
            </div>
          </div>

          <div className="flex justify-end gap-1.5 ml-6">
            <Button variant="outline" size="sm" className="h-6 px-2 text-[11px]" onClick={() => onOpenDetail(log)}>
              详情
            </Button>
            <Button
              size="sm"
              className="h-6 px-2 text-[11px]"
              onClick={() => onViewChatIO(log)}
              disabled={!canViewChatIO(log)}
            >
              会话
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-1.5 text-destructive"
              onClick={() => onOpenDelete(log.id)}
              disabled={deleting}
              aria-label="删除日志"
            >
              <Trash2 className="size-4" />
            </Button>
          </div>
        </StaggerItem>
      ))}
    </StaggerList>
  );
}
