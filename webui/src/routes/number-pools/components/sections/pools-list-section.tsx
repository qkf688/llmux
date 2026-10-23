import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { PoolListItem } from "@/lib/api";
import { PoolsTableBlock } from "./pools-table-block";
import { PoolsAutoSection } from "./pools-auto-section";

type PoolsListSectionProps = {
  /** 手建号池（主列表） */
  pools: PoolListItem[];
  /** 专属号池（折叠在主列表下方） */
  autoPools: PoolListItem[];
  isLoading?: boolean;
  isError?: boolean;
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  /** 请求删除（弹确认框），不直接删除 */
  onRequestDelete: (pool: PoolListItem) => void;
};

export function PoolsListSection({
  pools,
  autoPools,
  isLoading = false,
  isError = false,
  onOpenDetail,
  onEdit,
  onRequestDelete,
}: PoolsListSectionProps) {
  return (
    <TableCard>
      {isLoading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载号池列表" />
        </div>
      ) : isError ? (
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-destructive">
          加载号池失败，请刷新重试
        </div>
      ) : pools.length === 0 && autoPools.length === 0 ? (
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
          暂无号池数据
        </div>
      ) : (
        <div className="flex h-full flex-col">
          {pools.length === 0 ? (
            <div className="flex flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">
              暂无手建号池
            </div>
          ) : (
            <PoolsTableBlock
              pools={pools}
              onOpenDetail={onOpenDetail}
              onEdit={onEdit}
              onRequestDelete={onRequestDelete}
            />
          )}
          <PoolsAutoSection
            pools={autoPools}
            onOpenDetail={onOpenDetail}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
          />
        </div>
      )}
    </TableCard>
  );
}
