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
  if (isLoading) {
    return (
      <TableCard>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          加载号池…
        </div>
      </TableCard>
    );
  }

  if (isError) {
    return (
      <TableCard>
        <div className="flex flex-1 items-center justify-center text-sm text-destructive">
          加载号池失败，请刷新重试
        </div>
      </TableCard>
    );
  }

  if (pools.length === 0) {
    return (
      <TableCard>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          暂无号池，点击右上角「新建号池」创建
        </div>
      </TableCard>
    );
  }

  return (
    <TableCard>
      <PoolsTableDesktop pools={pools} onOpenDetail={onOpenDetail} onEdit={onEdit} onDelete={onDelete} />
      <PoolsMobileList pools={pools} onOpenDetail={onOpenDetail} onEdit={onEdit} onDelete={onDelete} />
    </TableCard>
  );
}
