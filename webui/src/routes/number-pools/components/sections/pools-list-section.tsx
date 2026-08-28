import { TableCard } from "@/components/table-card";
import type { MockPool } from "../../types";
import { PoolsTableDesktop } from "./pools-desktop-table";
import { PoolsMobileList } from "./pools-mobile-list";

type PoolsListSectionProps = {
  pools: MockPool[];
  onOpenDetail: (pool: MockPool) => void;
  onEdit: (pool: MockPool) => void;
  onDelete: (poolId: number) => void;
};

export function PoolsListSection({ pools, onOpenDetail, onEdit, onDelete }: PoolsListSectionProps) {
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