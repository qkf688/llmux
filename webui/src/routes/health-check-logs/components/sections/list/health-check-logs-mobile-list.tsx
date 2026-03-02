import { Button } from "@/components/ui/button";
import type { HealthCheckLog } from "@/lib/api";
import { formatDateTime, formatResponseTime, isHealthCheckSuccess } from "../../../utils/formatters";

type HealthCheckLogsMobileListProps = {
  logs: HealthCheckLog[];
  onOpenDetail: (log: HealthCheckLog) => void;
};

export function HealthCheckLogsMobileList({ logs, onOpenDetail }: HealthCheckLogsMobileListProps) {
  return (
    <div className="sm:hidden flex-1 px-2 py-3 divide-y divide-border overflow-y-auto">
      {logs.map((log) => {
        const success = isHealthCheckSuccess(log.status);

        return (
          <div key={log.ID} className="py-3 space-y-2 my-1 px-1">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <h3 className="font-semibold text-sm truncate">{log.model_name}</h3>
                <p className="text-[11px] text-muted-foreground">{formatDateTime(log.checked_at)}</p>
              </div>

              <div className="flex items-center gap-2">
                <span
                  className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${
                    success ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
                  }`}
                >
                  {success ? "成功" : "失败"}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => onOpenDetail(log)}
                >
                  详情
                </Button>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3 text-xs">
              <div className="space-y-1">
                <p className="text-muted-foreground text-[10px] uppercase tracking-wide">提供商</p>
                <p className="truncate">{log.provider_name}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-[10px] uppercase tracking-wide">响应时间</p>
                <p className="font-medium">{formatResponseTime(log.response_time)}</p>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
