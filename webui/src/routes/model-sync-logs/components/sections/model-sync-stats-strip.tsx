import { AlertTriangle, CheckCircle, Clock, Layers } from "lucide-react";
import type { ModelSyncStats } from "@/lib/api";
import { formatSyncDate } from "../../utils/formatters";

type ModelSyncStatsStripProps = {
  stats: ModelSyncStats | null;
  loading: boolean;
};

export function ModelSyncStatsStrip({ stats, loading }: ModelSyncStatsStripProps) {
  return (
    <div className="flex flex-wrap items-center gap-4 text-sm flex-shrink-0">
      {loading ? (
        <div className="h-6 w-48 bg-muted animate-pulse rounded" />
      ) : (
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <span>
            上次: {formatSyncDate(stats?.last_sync_at)}
            {stats?.sync_enabled && stats?.next_sync_at && <> · 下次: {formatSyncDate(stats.next_sync_at)}</>}
          </span>
          {stats?.sync_enabled ? (
            <span className="text-green-600">自动 {stats.sync_interval}h</span>
          ) : (
            <span className="text-muted-foreground">手动</span>
          )}
        </div>
      )}

      {loading ? (
        <div className="h-6 w-24 bg-muted animate-pulse rounded" />
      ) : (
        <div className="flex items-center gap-1">
          <Layers className="h-4 w-4 text-muted-foreground" />
          <span className="font-medium">{stats?.total_providers || 0}</span>
          <span className="text-muted-foreground">提供商</span>
        </div>
      )}

      {loading ? (
        <div className="h-6 w-16 bg-muted animate-pulse rounded" />
      ) : (
        <div className="flex items-center gap-1">
          <CheckCircle className="h-4 w-4 text-green-500" />
          <span className="font-medium text-green-600">{stats?.providers_with_updates || 0}</span>
          <span className="text-muted-foreground">更新</span>
        </div>
      )}

      {loading ? (
        <div className="h-6 w-16 bg-muted animate-pulse rounded" />
      ) : (
        <div className="flex items-center gap-1">
          <CheckCircle className="h-4 w-4 text-blue-500" />
          <span className="font-medium text-blue-600">{stats?.providers_unchanged || 0}</span>
          <span className="text-muted-foreground">无变</span>
        </div>
      )}

      {loading ? (
        <div className="h-6 w-16 bg-muted animate-pulse rounded" />
      ) : (
        <div className="flex items-center gap-1">
          <AlertTriangle className="h-4 w-4 text-orange-500" />
          <span className="font-medium text-orange-600">{stats?.providers_with_errors || 0}</span>
          <span className="text-muted-foreground">错误</span>
        </div>
      )}
    </div>
  );
}
