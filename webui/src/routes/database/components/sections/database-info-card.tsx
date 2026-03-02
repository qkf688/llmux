import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DatabaseStats } from "@/lib/api";

type DatabaseInfoCardProps = {
  stats: DatabaseStats;
};

export function DatabaseInfoCard({ stats }: DatabaseInfoCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>数据库信息</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        <div className="flex justify-between gap-4">
          <span className="text-muted-foreground flex-shrink-0">数据库路径:</span>
          <span className="font-mono break-all text-right">{stats.db_path}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">数据库版本:</span>
          <span>{stats.sqlite_version}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">编码:</span>
          <span>{stats.encoding}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">最后更新:</span>
          <span>{new Date(stats.last_modified).toLocaleString("zh-CN")}</span>
        </div>
      </CardContent>
    </Card>
  );
}
