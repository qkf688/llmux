import { Button } from "@/components/ui/button";
import { RefreshCw, Trash2 } from "lucide-react";

type LogsHeaderProps = {
  selectedCount: number;
  deleting: boolean;
  clearingAll: boolean;
  clearingFiltered: boolean;
  canClearFiltered: boolean;
  onRefresh: () => void;
  onOpenBatchDelete: () => void;
  onOpenClearAll: () => void;
  onOpenClearFiltered: () => void;
};

export function LogsHeader({
  selectedCount,
  deleting,
  clearingAll,
  clearingFiltered,
  canClearFiltered,
  onRefresh,
  onOpenBatchDelete,
  onOpenClearAll,
  onOpenClearFiltered,
}: LogsHeaderProps) {
  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="text-2xl font-bold tracking-tight">请求日志</h2>
        </div>
        <div className="flex gap-2 ml-auto">
          <Button
            variant="destructive"
            size="sm"
            className="shrink-0 h-7 px-2 text-xs gap-1 sm:h-8 sm:px-3 sm:text-sm sm:gap-1.5"
            disabled={clearingAll}
            onClick={onOpenClearAll}
          >
            <Trash2 className="size-3.5 mr-0.5 sm:size-4 sm:mr-1" />
            {clearingAll ? "清空中..." : "清空所有日志"}
          </Button>

          <Button
            variant="destructive"
            size="sm"
            className="shrink-0 h-7 px-2 text-xs gap-1 sm:h-8 sm:px-3 sm:text-sm sm:gap-1.5"
            disabled={!canClearFiltered || clearingFiltered}
            onClick={onOpenClearFiltered}
            title={!canClearFiltered ? "请先设置筛选条件" : undefined}
          >
            <Trash2 className="size-3.5 mr-0.5 sm:size-4 sm:mr-1" />
            {clearingFiltered ? "清空中..." : "清空筛选结果"}
          </Button>

          {selectedCount > 0 && (
            <Button
              onClick={onOpenBatchDelete}
              variant="destructive"
              size="sm"
              disabled={deleting}
              className="shrink-0"
            >
              <Trash2 className="size-4 mr-1" />
              删除 ({selectedCount})
            </Button>
          )}

          <Button
            onClick={onRefresh}
            variant="outline"
            size="icon"
            className="shrink-0"
            aria-label="刷新列表"
            title="刷新列表"
          >
            <RefreshCw className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
