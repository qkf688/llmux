import { RefreshCw, ScrollText, Trash2 } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

type LogsHeaderProps = {
  /** 日志总条数（全量，非当前页），用于页头副标题 */
  total: number;
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
  total,
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
    <PageHeader
      icon={ScrollText}
      title="请求日志"
      subtitle={`共 ${total} 条请求记录`}
      actions={
        <>
          <Button
            variant="destructive"
            size="sm"
            className="shrink-0"
            disabled={clearingAll}
            onClick={onOpenClearAll}
          >
            <Trash2 className="size-4" />
            {clearingAll ? "清空中..." : "清空所有日志"}
          </Button>

          <Button
            variant="destructive"
            size="sm"
            className="shrink-0"
            disabled={!canClearFiltered || clearingFiltered}
            onClick={onOpenClearFiltered}
            title={!canClearFiltered ? "请先设置筛选条件" : undefined}
          >
            <Trash2 className="size-4" />
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
              <Trash2 className="size-4" />
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
        </>
      }
    />
  );
}
