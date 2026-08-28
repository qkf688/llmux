import { Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { MockCredentialStatus, MockPool } from "../../types";

type PoolsDesktopTableProps = {
  pools: MockPool[];
  onOpenDetail: (pool: MockPool) => void;
  onEdit: (pool: MockPool) => void;
  onDelete: (poolId: number) => void;
};

/** 健康概览：四态计数彩色圆点（对应 CSS 变量 success/warning/destructive/muted） */
function HealthSummary({ pool }: { pool: MockPool }) {
  const count = (status: MockCredentialStatus) => pool.credentials.filter((c) => c.status === status).length;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
      <span className="inline-flex items-center gap-1">
        <span className="size-2 rounded-full bg-success" />
        {count("active")}
      </span>
      <span className="inline-flex items-center gap-1">
        <span className="size-2 rounded-full bg-warning" />
        {count("cooldown")}
      </span>
      <span className="inline-flex items-center gap-1">
        <span className="size-2 rounded-full bg-destructive" />
        {count("error")}
      </span>
      <span className="inline-flex items-center gap-1">
        <span className="size-2 rounded-full bg-muted-foreground/40" />
        {count("disabled")}
      </span>
    </div>
  );
}

export function PoolsTableDesktop({ pools, onOpenDetail, onEdit, onDelete }: PoolsDesktopTableProps) {
  return (
    <div className="hidden sm:block w-full overflow-x-auto">
      <Table className="min-w-[860px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
          <TableRow className="hover:bg-secondary/90">
            <TableHead>ID</TableHead>
            <TableHead>名称</TableHead>
            <TableHead>Keys</TableHead>
            <TableHead>健康概览</TableHead>
            <TableHead>被分组引用</TableHead>
            <TableHead className="w-[150px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pools.map((pool) => (
            <TableRow
              key={pool.id}
              className="cursor-pointer"
              tabIndex={0}
              onClick={() => onOpenDetail(pool)}
              onKeyDown={(e) => {
                if (e.key === "Enter") onOpenDetail(pool);
              }}
            >
              <TableCell className="font-mono text-xs text-muted-foreground">{pool.id}</TableCell>
              <TableCell>
                <div className="font-medium">{pool.name}</div>
                {pool.note && <div className="text-xs text-muted-foreground truncate max-w-[240px]">{pool.note}</div>}
              </TableCell>
              <TableCell className="text-xs">{pool.credentials.length}</TableCell>
              <TableCell>
                <HealthSummary pool={pool} />
              </TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {pool.refGroups.length === 0 ? (
                    <span className="text-xs text-muted-foreground">未引用</span>
                  ) : (
                    pool.refGroups.map((ref) => (
                      <Badge key={`${ref.providerName}.${ref.groupName}`} variant="outline" className="text-[10px]">
                        {ref.providerName} · {ref.groupName}
                      </Badge>
                    ))
                  )}
                </div>
              </TableCell>
              <TableCell onClick={(e) => e.stopPropagation()}>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2"
                    onClick={() => onOpenDetail(pool)}
                  >
                    详情
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2"
                    onClick={() => onEdit(pool)}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2 text-destructive hover:text-destructive"
                    onClick={() => onDelete(pool.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}