import { useEffect, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { ImportCredentialsDialog } from "./import-credentials-dialog";
import { CREDENTIAL_STATUS_LABEL } from "../../constants/credential-status";
import type { MockCredentialStatus, MockPool } from "../../types";

const PAGE_SIZE = 20;
const STATUS_OPTIONS: Array<"all" | MockCredentialStatus> = ["all", "active", "cooldown", "error", "disabled"];

const STATUS_BADGE_CLS: Record<MockCredentialStatus, string> = {
  active: "bg-success-tint text-success-foreground",
  cooldown: "bg-warning-tint text-warning-foreground",
  error: "bg-destructive-tint text-destructive-tint-foreground",
  disabled: "bg-muted text-muted-foreground",
};

function shortenKey(key: string): string {
  if (key.length <= 16) return key;
  return `${key.slice(0, 11)}…${key.slice(-4)}`;
}

type PoolDetailDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pool: MockPool | null;
  onPoolUpdate: (updated: MockPool) => void;
};

export function PoolDetailDialog({ open, onOpenChange, pool, onPoolUpdate }: PoolDetailDialogProps) {
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | MockCredentialStatus>("all");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [importOpen, setImportOpen] = useState(false);
  const [page, setPage] = useState(1);

  // 切换号池 / 开关弹窗时重置内部状态，避免串池
  useEffect(() => {
    setSearchText("");
    setStatusFilter("all");
    setSelectedIds(new Set());
    setImportOpen(false);
    setPage(1);
  }, [pool?.id, open]);

  const filtered = useMemo(() => {
    const kw = searchText.trim().toLowerCase();
    return (pool?.credentials ?? []).filter((c) => {
      if (statusFilter !== "all" && c.status !== statusFilter) return false;
      if (kw && !c.key.toLowerCase().includes(kw) && !(c.note ?? "").toLowerCase().includes(kw)) return false;
      return true;
    });
  }, [pool, searchText, statusFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paged = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
  const isAllSelected = paged.length > 0 && paged.every((c) => selectedIds.has(c.id));
  const isSomeSelected = paged.some((c) => selectedIds.has(c.id));

  // 删除/筛选导致总页数缩小时钳制页码，避免「6 / 5」死页
  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  if (!pool) {
    return null;
  }

  const count = (status: MockCredentialStatus) => pool.credentials.filter((c) => c.status === status).length;

  const handleImportClick = (text: string): { imported: number; skipped: number } => {
    const lines = text
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (lines.length === 0) return { imported: 0, skipped: 0 };
    const existing = new Set(pool.credentials.map((c) => c.key));
    // 同批内重复也要剔除（粘 sk-a\nsk-a 只算一条）；skipped = 库内已有 + 批内重复
    const seen = new Set(existing);
    const fresh = lines.filter((k) => {
      if (seen.has(k)) return false;
      seen.add(k);
      return true;
    });
    const duplicated = lines.length - fresh.length;
    const baseId = pool.credentials.length > 0 ? Math.max(...pool.credentials.map((c) => c.id)) : 0;
    onPoolUpdate({
      ...pool,
      credentials: [
        ...pool.credentials,
        ...fresh.map((key, i) => ({
          id: baseId + i + 1,
          key,
          status: "active" as const,
          failCount: 0,
          lastUsedAt: "刚刚",
          totalRequests: 0,
          totalErrors: 0,
        })),
      ],
    });
    setPage(1);
    if (fresh.length > 0) {
      toast.success(`已导入 ${fresh.length} 条凭据（原型）`);
    }
    return { imported: fresh.length, skipped: duplicated };
  };

  const applyBulk = (next: MockCredentialStatus | "delete") => {
    if (selectedIds.size === 0 || !pool) return;
    const ids = selectedIds;
    if (next === "delete") {
      onPoolUpdate({ ...pool, credentials: pool.credentials.filter((c) => !ids.has(c.id)) });
      toast.success(`已删除 ${ids.size} 条凭据（原型）`);
    } else {
      onPoolUpdate({
        ...pool,
        credentials: pool.credentials.map((c) => (ids.has(c.id) ? { ...c, status: next } : c)),
      });
      toast.success(`已${next === "active" ? "启用" : "停用"} ${ids.size} 条凭据（原型）`);
    }
    setSelectedIds(new Set());
  };

  const setCredentialStatus = (id: number, status: MockCredentialStatus) => {
    onPoolUpdate({
      ...pool,
      credentials: pool.credentials.map((c) => (c.id === id ? { ...c, status } : c)),
    });
  };

  const deleteCredential = (id: number) => {
    onPoolUpdate({ ...pool, credentials: pool.credentials.filter((c) => c.id !== id) });
    setSelectedIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    toast.success("已删除凭据（原型）");
  };

  const toggleSelected = (id: number, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const toggleAllInPage = (checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      paged.forEach((c) => {
        if (checked) next.add(c.id);
        else next.delete(c.id);
      });
      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="xl" className="p-4">
        <div className="p-3 border-b flex-shrink-0 sm:p-4">
          <DialogHeader className="p-0">
            <DialogTitle>
              号池：{pool.name}
              <Badge variant="outline" className="ml-2 text-[10px] align-middle">
                S0 原型
              </Badge>
            </DialogTitle>
            <DialogDescription>
              {pool.note ? `${pool.note} · ` : ""}凭据管理：批量导入 / 状态筛选 / 批量启停（共 {pool.credentials.length} 条）
            </DialogDescription>
          </DialogHeader>
        </div>

        {/* 固定工具区：健康概览 / 搜索筛选工具栏（不随表格滚动） */}
        <div className="flex-shrink-0 space-y-3 text-sm">
            {/* 健康概览 */}
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md border bg-muted/20 p-2 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1">
                <span className="size-2 rounded-full bg-success" />
                健康 {count("active")}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="size-2 rounded-full bg-warning" />
                冷却 {count("cooldown")}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="size-2 rounded-full bg-destructive" />
                错误 {count("error")}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="size-2 rounded-full bg-muted-foreground/40" />
                停用 {count("disabled")}
              </span>
            </div>

            {/* 工具栏：搜索 / 状态筛选 / 批量操作 / 添加入口 */}
            <div className="flex flex-wrap items-center gap-2">
              <Input
                className="w-56"
                placeholder="搜索 Key / 备注"
                value={searchText}
                onChange={(e) => {
                  setSearchText(e.target.value);
                  setPage(1);
                }}
              />
              <select
                className="flex h-10 w-32 rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={statusFilter}
                onChange={(e) => {
                  setStatusFilter(e.target.value as "all" | MockCredentialStatus);
                  setPage(1);
                }}
              >
                {STATUS_OPTIONS.map((s) => (
                  <option key={s} value={s}>
                    {s === "all" ? "全部状态" : CREDENTIAL_STATUS_LABEL[s]}
                  </option>
                ))}
              </select>
              {selectedIds.size > 0 && (
                <div className="flex items-center gap-1">
                  <span className="text-xs text-muted-foreground">已选 {selectedIds.size} 条</span>
                  <Button size="sm" variant="outline" onClick={() => applyBulk("active")}>
                    启用
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => applyBulk("disabled")}>
                    停用
                  </Button>
                  <Button size="sm" variant="destructive" onClick={() => applyBulk("delete")}>
                    删除
                  </Button>
                </div>
              )}
              <Button size="sm" className="ml-auto" onClick={() => setImportOpen(true)}>
                <Plus className="size-4" />
                添加
              </Button>
            </div>
        </div>

        {/* 滚动区：仅凭据表格与移动列表 */}
        <DialogBody className="flex flex-col gap-3 p-3 text-sm">
            {/* 凭据表格（桌面） */}
            <div className="hidden sm:block overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[40px]">
                      <Checkbox
                        checked={isSomeSelected ? "indeterminate" : isAllSelected}
                        onCheckedChange={(checked) => toggleAllInPage(checked === true)}
                        aria-label="全选当前页"
                      />
                    </TableHead>
                    <TableHead>Key</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>备注</TableHead>
                    <TableHead>失败</TableHead>
                    <TableHead>最近使用</TableHead>
                    <TableHead>请求/错误</TableHead>
                    <TableHead className="w-[120px]">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {paged.map((cred) => (
                    <TableRow key={cred.id}>
                      <TableCell>
                        <Checkbox
                          checked={selectedIds.has(cred.id)}
                          onCheckedChange={(checked) => toggleSelected(cred.id, checked === true)}
                          aria-label={`选择凭据 ${cred.id}`}
                        />
                      </TableCell>
                      <TableCell className="font-mono text-xs" title={cred.key}>
                        {shortenKey(cred.key)}
                      </TableCell>
                      <TableCell>
                        <span
                          className={`inline-flex rounded-full px-2 py-0.5 text-[10.5px] font-medium ${STATUS_BADGE_CLS[cred.status]}`}
                        >
                          {CREDENTIAL_STATUS_LABEL[cred.status]}
                        </span>
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate text-xs text-muted-foreground" title={cred.note}>
                        {cred.note || "-"}
                      </TableCell>
                      <TableCell className="text-xs">{cred.failCount}</TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                        {cred.lastUsedAt ?? "-"}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {cred.totalRequests} / {cred.totalErrors}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 px-2 text-xs"
                            onClick={() => setCredentialStatus(cred.id, cred.status === "disabled" ? "active" : "disabled")}
                          >
                            {cred.status === "disabled" ? "启用" : "停用"}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                            onClick={() => deleteCredential(cred.id)}
                          >
                            删除
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            {/* 凭据列表（移动） */}
            <div className="sm:hidden space-y-2">
              {paged.map((cred) => (
                <div key={cred.id} className="flex items-start gap-2 rounded-md border p-2">
                  <Checkbox
                    checked={selectedIds.has(cred.id)}
                    onCheckedChange={(checked) => toggleSelected(cred.id, checked === true)}
                    className="mt-0.5 shrink-0"
                    aria-label={`选择凭据 ${cred.id}`}
                  />
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-[11px] truncate">{shortenKey(cred.key)}</span>
                      <span
                        className={`shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium ${STATUS_BADGE_CLS[cred.status]}`}
                      >
                        {CREDENTIAL_STATUS_LABEL[cred.status]}
                      </span>
                    </div>
                    <p className="text-[10px] text-muted-foreground">
                      {cred.note || "-"} · 失败 {cred.failCount} · 最近 {cred.lastUsedAt ?? "-"}
                    </p>
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-2 text-[11px]"
                        onClick={() => setCredentialStatus(cred.id, cred.status === "disabled" ? "active" : "disabled")}
                      >
                        {cred.status === "disabled" ? "启用" : "停用"}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-2 text-[11px] text-destructive hover:text-destructive"
                        onClick={() => deleteCredential(cred.id)}
                      >
                        删除
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
        </DialogBody>

        {/* 固定底部：被引用分组（纯展示；双向导航深链待 S6 接真实 API 后实现）/ 分页，一行两端对齐 */}
        <div className="flex-shrink-0 flex flex-wrap items-center justify-between gap-2 border-t px-1 pt-3">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {pool.refGroups.length === 0 ? (
              <span>尚未被任何分组引用</span>
            ) : (
              <>
                <span className="shrink-0">被引用</span>
                {pool.refGroups.map((ref) => (
                  <span
                    key={`${ref.providerName}.${ref.groupName}`}
                    className="inline-flex items-center gap-1 rounded-full border bg-background px-2 py-0.5 text-[11px] text-foreground"
                  >
                    <span className="font-medium">{ref.providerName}</span>
                    <span className="text-muted-foreground">· {ref.groupName}</span>
                  </span>
                ))}
              </>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span>
              共 {filtered.length} 条{filtered.length !== pool.credentials.length ? "（筛选后）" : ""}
            </span>
            <div className="flex items-center gap-1">
              <Button size="sm" variant="outline" className="h-7 px-2" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                上一页
              </Button>
              <span>
                {page} / {totalPages}
              </span>
              <Button
                size="sm"
                variant="outline"
                className="h-7 px-2"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
      <ImportCredentialsDialog open={importOpen} onOpenChange={setImportOpen} onImport={handleImportClick} />
    </Dialog>
  );
}