import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DatabaseStats } from "@/lib/api";
import { Database, HardDrive, RefreshCw, Table as TableIcon } from "lucide-react";

type DatabaseStatsCardsProps = {
  stats: DatabaseStats;
  usageRate: string;
};

export function DatabaseStatsCards({ stats, usageRate }: DatabaseStatsCardsProps) {
  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-2 sm:gap-4">
      <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-1 sm:pb-2 px-3 sm:px-6">
          <CardTitle className="text-xs sm:text-sm font-medium">文件大小</CardTitle>
          <HardDrive className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="px-3 sm:px-6">
          <div className="text-lg sm:text-2xl font-bold leading-none">{stats.file_size_human}</div>
          <p className="hidden sm:block text-xs text-muted-foreground">{stats.file_size.toLocaleString()} 字节</p>
        </CardContent>
      </Card>

      <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-1 sm:pb-2 px-3 sm:px-6">
          <CardTitle className="text-xs sm:text-sm font-medium">使用率</CardTitle>
          <Database className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="px-3 sm:px-6">
          <div className="text-lg sm:text-2xl font-bold leading-none">{usageRate}%</div>
          <p className="hidden sm:block text-xs text-muted-foreground">
            {stats.page_count - stats.free_pages} / {stats.page_count} 页
          </p>
        </CardContent>
      </Card>

      <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-1 sm:pb-2 px-3 sm:px-6">
          <CardTitle className="text-xs sm:text-sm font-medium">空闲空间</CardTitle>
          <RefreshCw className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="px-3 sm:px-6">
          <div className="text-lg sm:text-2xl font-bold leading-none">{stats.free_pages} 页</div>
          <p className="hidden sm:block text-xs text-muted-foreground">
            {(stats.free_pages * stats.page_size).toLocaleString()} 字节
          </p>
        </CardContent>
      </Card>

      <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-1 sm:pb-2 px-3 sm:px-6">
          <CardTitle className="text-xs sm:text-sm font-medium">页面大小</CardTitle>
          <TableIcon className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="px-3 sm:px-6">
          <div className="text-lg sm:text-2xl font-bold leading-none">{stats.page_size.toLocaleString()}</div>
          <p className="hidden sm:block text-xs text-muted-foreground">字节/页</p>
        </CardContent>
      </Card>
    </div>
  );
}
