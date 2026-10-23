import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { PoolListItem } from "@/lib/api";
import { PoolsTableBlock } from "./pools-table-block";

type PoolsAutoSectionProps = {
  pools: PoolListItem[];
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  onRequestDelete: (pool: PoolListItem) => void;
};

/**
 * 系统自动创建的号池折叠区。
 *
 * 这些池是供应商分组点「添加 Key」时自动建出来的专属池（pools.auto_created=true），
 * 数量多、平时不需要逐个看；但没人引用的孤儿池要能在这里找到并手动删掉，
 * 所以默认折叠、点标题展开，而不是从列表里藏起来。
 */
export function PoolsAutoSection({
  pools,
  onOpenDetail,
  onEdit,
  onRequestDelete,
}: PoolsAutoSectionProps) {
  const [expanded, setExpanded] = useState(false);
  if (pools.length === 0) return null;

  return (
    <div className="shrink-0 border-t">
      <button
        type="button"
        aria-expanded={expanded}
        className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm font-medium hover:bg-muted/50"
        onClick={() => setExpanded((prev) => !prev)}
      >
        {expanded ? (
          <ChevronDown className="size-4 shrink-0" />
        ) : (
          <ChevronRight className="size-4 shrink-0" />
        )}
        系统自动创建的号池
        <span className="text-xs font-normal text-muted-foreground">{pools.length} 个</span>
        <span className="text-xs font-normal text-muted-foreground">
          （供应商分组「添加 Key」时自动建出，专属该分组）
        </span>
      </button>
      {expanded && (
        <div className="max-h-[40vh] overflow-auto border-t">
          <PoolsTableBlock
            pools={pools}
            onOpenDetail={onOpenDetail}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
          />
        </div>
      )}
    </div>
  );
}
