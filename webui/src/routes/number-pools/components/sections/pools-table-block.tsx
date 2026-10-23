import type { PoolListItem } from "@/lib/api";
import { PoolsTableDesktop } from "./pools-desktop-table";
import { PoolsMobileList } from "./pools-mobile-list";

type PoolsTableBlockProps = {
  pools: PoolListItem[];
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  onRequestDelete: (pool: PoolListItem) => void;
};

/**
 * 号池表格块：桌面表格 + 移动列表两套视图共用同一份 props。
 * 主列表与「专属号池」折叠区都渲染它，避免两处各写一遍
 * 「桌面表格 + 移动列表」的组合（改一处漏另一处 = 两块区的行为漂移）。
 */
export function PoolsTableBlock({
  pools,
  onOpenDetail,
  onEdit,
  onRequestDelete,
}: PoolsTableBlockProps) {
  return (
    <>
      <PoolsTableDesktop
        pools={pools}
        onOpenDetail={onOpenDetail}
        onEdit={onEdit}
        onRequestDelete={onRequestDelete}
      />
      <PoolsMobileList
        pools={pools}
        onOpenDetail={onOpenDetail}
        onEdit={onEdit}
        onRequestDelete={onRequestDelete}
      />
    </>
  );
}
