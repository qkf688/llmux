import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import type { ChatLog } from "@/lib/api";
import { Trash2 } from "lucide-react";
import { formatByteLength, formatDateTime, formatTime } from "../../../utils/formatters";

type LogsMobileListProps = {
  logs: ChatLog[];
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
  selectedIds,
  deleting,
  canViewChatIO,
  onSelectOne,
  onOpenDetail,
  onViewChatIO,
  onOpenDelete,
}: LogsMobileListProps) {
  return (
    <div className="sm:hidden px-2 py-3 divide-y divide-border overflow-y-auto">
      {logs.map((log) => (
        <div
          key={log.ID}
          className={`py-3 space-y-2 my-1 px-1 ${selectedIds.has(log.ID) ? "bg-muted/50 rounded" : ""}`}
        >
          <div className="flex items-start justify-between gap-2">
            <div className="flex items-start gap-2 min-w-0 flex-1">
              <Checkbox
                checked={selectedIds.has(log.ID)}
                onCheckedChange={(checked) => onSelectOne(log.ID, checked === true)}
                className="mt-1"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-sm truncate">{log.Name}</h3>
                  {log.is_virtual_model && (
                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0.5">
                      虚拟
                    </Badge>
                  )}
                </div>
                <p className="text-[11px] text-muted-foreground">{formatDateTime(log.CreatedAt)}</p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <span
                className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${
                  log.Status === "success" ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
                }`}
              >
                {log.Status}
              </span>
              <div className="flex gap-1">
                <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => onOpenDetail(log)}>
                  详情
                </Button>
                <Button
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => onViewChatIO(log)}
                  disabled={!canViewChatIO(log)}
                >
                  会话
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs text-destructive"
                  onClick={() => onOpenDelete(log.ID)}
                  disabled={deleting}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 text-xs ml-6">
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">耗时</p>
              <p className="font-medium">{formatTime(log.ChunkTime)}</p>
            </div>
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">提供商</p>
              <p className="truncate">{log.ProviderName}</p>
            </div>
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">格式转换</p>
              <p>
                {log.has_format_conversion ? (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0.5">
                    {log.source_format} → {log.target_format}
                  </Badge>
                ) : (
                  <span className="text-muted-foreground">-</span>
                )}
              </p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 text-xs ml-6">
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">请求头</p>
              <p className="font-medium">{formatByteLength(log.RequestHeaders)}</p>
            </div>
            <div className="space-y-1">
              <p className="text-muted-foreground text-[10px] uppercase tracking-wide">响应头</p>
              <p className="font-medium">{formatByteLength(log.ResponseHeaders)}</p>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
