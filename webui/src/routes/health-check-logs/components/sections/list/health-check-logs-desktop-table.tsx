import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { HealthCheckLog } from "@/lib/api";
import { formatDateTime, formatResponseTime, isHealthCheckSuccess } from "../../../utils/formatters";

type HealthCheckLogsDesktopTableProps = {
  logs: HealthCheckLog[];
  onOpenDetail: (log: HealthCheckLog) => void;
};

export function HealthCheckLogsDesktopTable({
  logs,
  onOpenDetail,
}: HealthCheckLogsDesktopTableProps) {
  return (
    <div className="flex-1 overflow-y-auto hidden sm:block w-full">
      <Table className="min-w-[900px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
          <TableRow className="hover:bg-secondary/90">
            <TableHead>检测时间</TableHead>
            <TableHead>模型名称</TableHead>
            <TableHead>提供商</TableHead>
            <TableHead>提供商模型</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>响应时间</TableHead>
            <TableHead className="w-[100px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => {
            const success = isHealthCheckSuccess(log.status);

            return (
              <TableRow key={log.ID}>
                <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                  {formatDateTime(log.checked_at)}
                </TableCell>
                <TableCell className="font-medium">{log.model_name}</TableCell>
                <TableCell className="text-xs">{log.provider_name}</TableCell>
                <TableCell className="max-w-[150px] truncate text-xs" title={log.provider_model}>
                  {log.provider_model}
                </TableCell>
                <TableCell>
                  <span
                    className={`inline-flex items-center px-2 py-1 ${
                      success ? "text-green-500" : "text-red-500"
                    }`}
                  >
                    {success ? "成功" : "失败"}
                  </span>
                </TableCell>
                <TableCell>{formatResponseTime(log.response_time)}</TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2"
                    onClick={() => onOpenDetail(log)}
                  >
                    详情
                  </Button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
