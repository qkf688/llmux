import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import type { PoolListItem } from "@/lib/api";
import {
  CREDENTIAL_STATUS_DOT_CLS,
  CREDENTIAL_STATUS_LABEL,
  CREDENTIAL_STATUS_ORDER,
} from "@/components/credentials/credential-status";

type PoolsMobileListProps = {
  pools: PoolListItem[];
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  onRequestDelete: (pool: PoolListItem) => void;
};

export function PoolsMobileList({ pools, onOpenDetail, onEdit, onRequestDelete }: PoolsMobileListProps) {
  return (
    <StaggerList className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border">
      {pools.map((pool) => {
        const counts = pool.StatusCounts ?? {};
        return (
          <StaggerItem key={pool.ID} className="py-2 space-y-2 my-0.5 px-1 cursor-pointer">
            <div className="flex items-start justify-between gap-2" onClick={() => onOpenDetail(pool)}>
              <div className="min-w-0">
                <h3 className="font-semibold text-[13px] leading-snug break-words">{pool.Name}</h3>
                {pool.Note && <p className="text-[11px] text-muted-foreground truncate">{pool.Note}</p>}
              </div>
              <Badge variant="secondary" className="shrink-0 text-[10px] px-1.5 py-0.5">
                {pool.KeyCount} keys
              </Badge>
            </div>
            <div className="flex items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
              {CREDENTIAL_STATUS_ORDER.map((s) => (
                <span key={s} className="inline-flex items-center gap-1" title={CREDENTIAL_STATUS_LABEL[s]}>
                  <span className={`size-2 rounded-full ${CREDENTIAL_STATUS_DOT_CLS[s]}`} />
                  {counts[s] ?? 0}
                </span>
              ))}
            </div>
            {/* 引用标记与桌面表格同口径：没人引用的池（孤儿专属池）在移动端也要看得出来，
                否则用户无从判断哪个池该手动删（本仓不做常驻清理，手动删是唯一出口）。 */}
            {pool.ReferencedBy > 0 ? (
              <p className="text-[10px] text-muted-foreground">被 {pool.ReferencedBy} 个分组引用</p>
            ) : (
              <p className="text-[10px] text-muted-foreground">未引用</p>
            )}
            <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => onOpenDetail(pool)}>
                详情
              </Button>
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => onEdit(pool)}>
                编辑
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                onClick={() => onRequestDelete(pool)}
              >
                删除
              </Button>
            </div>
          </StaggerItem>
        );
      })}
    </StaggerList>
  );
}
