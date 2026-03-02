import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DatabaseStats } from "@/lib/api";

type DatabaseInfoCardProps = {
  stats: DatabaseStats;
};

export function DatabaseInfoCard({ stats }: DatabaseInfoCardProps) {
  return (
    <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
      <CardHeader className="px-3 sm:px-6">
        <CardTitle className="text-sm sm:text-base">数据库信息</CardTitle>
      </CardHeader>
      <CardContent className="px-3 sm:px-6 space-y-1.5 sm:space-y-2 text-xs sm:text-sm">
        <div className="flex justify-between gap-3">
          <span className="text-muted-foreground flex-shrink-0">数据库路径:</span>
          <span
            className="font-mono text-right flex-1 min-w-0 truncate sm:whitespace-normal sm:break-all sm:overflow-visible"
            title={stats.db_path}
          >
            {stats.db_path}
          </span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-muted-foreground">数据库版本:</span>
          <span>{stats.sqlite_version}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-muted-foreground">编码:</span>
          <span>{stats.encoding}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-muted-foreground">最后更新:</span>
          <span>{new Date(stats.last_modified).toLocaleString("zh-CN")}</span>
        </div>
      </CardContent>
    </Card>
  );
}
