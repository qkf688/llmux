import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import type { MockCredentialStatus, MockPool } from "../../types";

type PoolsMobileListProps = {
  pools: MockPool[];
  onOpenDetail: (pool: MockPool) => void;
  onEdit: (pool: MockPool) => void;
  onDelete: (poolId: number) => void;
};

const STATUS_DOT_CLS: Record<MockCredentialStatus, string> = {
  active: "bg-success",
  cooldown: "bg-warning",
  error: "bg-destructive",
  disabled: "bg-muted-foreground/40",
};

export function PoolsMobileList({ pools, onOpenDetail, onEdit, onDelete }: PoolsMobileListProps) {
  return (
    <StaggerList className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border">
      {pools.map((pool) => {
        const count = (status: MockCredentialStatus) => pool.credentials.filter((c) => c.status === status).length;
        return (
          <StaggerItem key={pool.id} className="py-2 space-y-2 my-0.5 px-1 cursor-pointer" >
            <div className="flex items-start justify-between gap-2" onClick={() => onOpenDetail(pool)}>
              <div className="min-w-0">
                <h3 className="font-semibold text-[13px] leading-snug break-words">{pool.name}</h3>
                {pool.note && <p className="text-[11px] text-muted-foreground truncate">{pool.note}</p>}
              </div>
              <Badge variant="secondary" className="shrink-0 text-[10px] px-1.5 py-0.5">
                {pool.credentials.length} keys
              </Badge>
            </div>
            <div className="flex items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
              {(["active", "cooldown", "error", "disabled"] as MockCredentialStatus[]).map((s) => (
                <span key={s} className="inline-flex items-center gap-1">
                  <span className={`size-2 rounded-full ${STATUS_DOT_CLS[s]}`} />
                  {count(s)}
                </span>
              ))}
            </div>
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
                onClick={() => onDelete(pool.id)}
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