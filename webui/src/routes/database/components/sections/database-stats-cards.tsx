import { StatCard, StatGrid } from "@/components/stat-card";
import type { DatabaseStats } from "@/lib/api";

type DatabaseStatsCardsProps = {
  stats: DatabaseStats;
  usageRate: string;
};

export function DatabaseStatsCards({ stats, usageRate }: DatabaseStatsCardsProps) {
  return (
    <StatGrid>
      <StatCard label="文件大小" value={stats.file_size_human} sub={`${stats.file_size.toLocaleString()} 字节`} />
      <StatCard
        label="使用率"
        value={usageRate}
        unit="%"
        sub={`${stats.page_count - stats.free_pages} / ${stats.page_count} 页`}
      />
      <StatCard
        label="空闲空间"
        value={stats.free_pages}
        unit="页"
        sub={`${(stats.free_pages * stats.page_size).toLocaleString()} 字节`}
      />
      <StatCard label="页面大小" value={stats.page_size.toLocaleString()} sub="字节/页" />
    </StatGrid>
  );
}
