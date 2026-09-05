import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { PoolListItem } from "@/lib/api";
import { PoolsTableDesktop } from "./pools-desktop-table";
import { PoolsMobileList } from "./pools-mobile-list";

type PoolsListSectionProps = {
  pools: PoolListItem[];
  isLoading?: boolean;
  isError?: boolean;
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  onDelete: (poolId: number) => void;
};

export function PoolsListSection({
  pools,
  isLoading = false,
  isError = false,
  onOpenDetail,
  onEdit,
  onDelete,
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
      ) : pools.length === 0 ? (
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
          暂无号池数据
        </div>
      ) : (
        <div className="flex h-full flex-col">
          <PoolsTableDesktop pools={pools} onOpenDetail={onOpenDetail} onEdit={onEdit} onDelete={onDelete} />
          <PoolsMobileList pools={pools} onOpenDetail={onOpenDetail} onEdit={onEdit} onDelete={onDelete} />
        </div>
      )}
    </TableCard>
  );
}
