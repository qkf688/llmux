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
import type { PoolListItem } from "@/lib/api";
import {
  CREDENTIAL_STATUS_DOT_CLS,
  CREDENTIAL_STATUS_LABEL,
  CREDENTIAL_STATUS_ORDER,
} from "../../constants/credential-status";

type PoolsDesktopTableProps = {
  pools: PoolListItem[];
  onOpenDetail: (pool: PoolListItem) => void;
  onEdit: (pool: PoolListItem) => void;
  onDelete: (poolId: number) => void;
};

function HealthSummary({ pool }: { pool: PoolListItem }) {
  const counts = pool.StatusCounts ?? {};
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
      {CREDENTIAL_STATUS_ORDER.map((status) => (
        <span key={status} className="inline-flex items-center gap-1" title={CREDENTIAL_STATUS_LABEL[status]}>
          <span className={`size-2 rounded-full ${CREDENTIAL_STATUS_DOT_CLS[status]}`} />
          {counts[status] ?? 0}
        </span>
      ))}
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
              key={pool.ID}
              className="cursor-pointer"
              onClick={() => onOpenDetail(pool)}
            >
              <TableCell className="font-mono text-xs text-muted-foreground">{pool.ID}</TableCell>
              <TableCell>
                <div className="font-medium">{pool.Name}</div>
                {pool.Note && (
                  <div className="text-xs text-muted-foreground truncate max-w-[240px]">{pool.Note}</div>
                )}
              </TableCell>
              <TableCell className="text-xs">{pool.KeyCount}</TableCell>
              <TableCell>
                <HealthSummary pool={pool} />
              </TableCell>
              <TableCell>
                {pool.ReferencedBy === 0 ? (
                  <span className="text-xs text-muted-foreground">未引用</span>
                ) : (
                  <Badge variant="outline" className="text-[10px]">
                    {pool.ReferencedBy} 个分组
                  </Badge>
                )}
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
                    onClick={() => onDelete(pool.ID)}
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
