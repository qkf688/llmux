import { useEffect, useState } from "react";
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
import {
  usePoolCredentials,
  useUpdateCredential,
  useDeleteCredential,
  useBatchUpdateCredentialStatus,
  useBatchDeleteCredentials,
  useBatchImportCredentials,
} from "@/hooks/api";
import type { CredentialStatus, PoolListItem } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import { ImportCredentialsDialog } from "./import-credentials-dialog";
import {
  COOLDOWN_BADGE_CLS,
  CREDENTIAL_STATUS_BADGE_CLS,
  CREDENTIAL_STATUS_DOT_CLS,
  CREDENTIAL_STATUS_LABEL,
  CREDENTIAL_STATUS_ORDER,
  formatCredentialTime,
  isCredentialInCooldown,
} from "./credential-status";

const PAGE_SIZE = 20;
// 筛选下拉 = 「全部」+ 状态顺序单表派生：状态清单只活在 credential-status.ts 一处
const STATUS_FILTER_OPTIONS: Array<"all" | CredentialStatus> = ["all", ...CREDENTIAL_STATUS_ORDER];

type PoolDetailDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pool: PoolListItem | null;
};

export function PoolDetailDialog({ open, onOpenChange, pool }: PoolDetailDialogProps) {
  const [searchText, setSearchText] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | CredentialStatus>("all");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [importOpen, setImportOpen] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    setSearchText("");
    setDebouncedQ("");
    setStatusFilter("all");
    setSelectedIds(new Set());
    setImportOpen(false);
    setPage(1);
  }, [pool?.ID, open]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedQ(searchText.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(handle);
  }, [searchText]);

  // 筛选 / 搜索 / 翻页后清空勾选，避免对不可见行做批量操作
  useEffect(() => {
    setSelectedIds(new Set());
  }, [statusFilter, debouncedQ, page]);

  const poolId = pool?.ID ?? null;
  const { data, isLoading, isFetching } = usePoolCredentials(open ? poolId : null, {
    page,
    page_size: PAGE_SIZE,
    status: statusFilter,
    q: debouncedQ || undefined,
  });

  const updateCred = useUpdateCredential();
  const deleteCred = useDeleteCredential();
  const batchStatus = useBatchUpdateCredentialStatus();
  const batchDelete = useBatchDeleteCredentials();
  const batchImport = useBatchImportCredentials();

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const isAllSelected = items.length > 0 && items.every((c) => selectedIds.has(c.ID));
  const isSomeSelected = items.some((c) => selectedIds.has(c.ID)) && !isAllSelected;

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  if (!pool) {
    return null;
  }

  const statusCount = (status: CredentialStatus) => pool.StatusCounts?.[status] ?? 0;

  const handleImport = async (text: string) => {
    const keys = text
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (keys.length === 0) return null;
    try {
      const result = await batchImport.mutateAsync({ poolId: pool.ID, keys });
      if (result.imported > 0) {
        toast.success(`已导入 ${result.imported} 条凭据`);
      }
      setPage(1);
      setSelectedIds(new Set());
      return result;
    } catch (err) {
      toast.error(`导入失败: ${toErrorMessage(err)}`);
      return null;
    }
  };

  const applyBulk = async (next: "active" | "disabled" | "delete") => {
    if (selectedIds.size === 0) return;
    const ids = Array.from(selectedIds);
    try {
      if (next === "delete") {
        await batchDelete.mutateAsync({ poolId: pool.ID, ids });
        toast.success(`已删除 ${ids.length} 条凭据`);
      } else {
        await batchStatus.mutateAsync({ poolId: pool.ID, ids, status: next });
        toast.success(`已${next === "active" ? "启用" : "停用"} ${ids.length} 条凭据`);
      }
      setSelectedIds(new Set());
    } catch (err) {
      toast.error(`批量操作失败: ${toErrorMessage(err)}`);
    }
  };

  const setCredentialStatus = async (id: number, status: CredentialStatus) => {
    try {
      await updateCred.mutateAsync({ poolId: pool.ID, credId: id, data: { status } });
    } catch (err) {
      toast.error(`更新失败: ${toErrorMessage(err)}`);
    }
  };

  const handleDeleteCredential = async (id: number) => {
    try {
      await deleteCred.mutateAsync({ poolId: pool.ID, credId: id });
      setSelectedIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      toast.success("已删除凭据");
    } catch (err) {
      toast.error(`删除失败: ${toErrorMessage(err)}`);
    }
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
      items.forEach((c) => {
        if (checked) next.add(c.ID);
        else next.delete(c.ID);
      });
      return next;
    });
  };

  const renderStatusBadges = (status: string, cooldownUntil: string | null) => {
    const known = status in CREDENTIAL_STATUS_LABEL ? (status as CredentialStatus) : null;
    return (
      <div className="flex flex-wrap items-center gap-1">
        {known ? (
          <span
            className={`inline-flex rounded-full px-2 py-0.5 text-[10.5px] font-medium ${CREDENTIAL_STATUS_BADGE_CLS[known]}`}
          >
            {CREDENTIAL_STATUS_LABEL[known]}
          </span>
        ) : (
          <span className="inline-flex rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
            {status}
          </span>
        )}
        {isCredentialInCooldown(cooldownUntil) && (
          <span className={`inline-flex rounded-full px-2 py-0.5 text-[10.5px] font-medium ${COOLDOWN_BADGE_CLS}`}>
            冷却
          </span>
        )}
      </div>
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="xl" className="p-4">
        <div className="p-3 border-b flex-shrink-0 sm:p-4">
          <DialogHeader className="p-0">
            <DialogTitle>号池：{pool.Name}</DialogTitle>
            <DialogDescription>
              {pool.Note ? `${pool.Note} · ` : ""}凭据管理：批量导入 / 状态筛选 / 批量启停（共 {pool.KeyCount} 条）
            </DialogDescription>
          </DialogHeader>
        </div>

        <div className="flex-shrink-0 space-y-3 text-sm">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md border bg-muted/20 p-2 text-xs text-muted-foreground">
            {CREDENTIAL_STATUS_ORDER.map((s) => (
              <span key={s} className="inline-flex items-center gap-1">
                <span className={`size-2 rounded-full ${CREDENTIAL_STATUS_DOT_CLS[s]}`} />
                {CREDENTIAL_STATUS_LABEL[s]} {statusCount(s)}
              </span>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Input
              className="w-56"
              placeholder="搜索备注"
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
            />
            <select
              className="flex h-10 w-36 rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value as "all" | CredentialStatus);
                setPage(1);
              }}
            >
              {STATUS_FILTER_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s === "all" ? "全部状态" : CREDENTIAL_STATUS_LABEL[s]}
                </option>
              ))}
            </select>
            {selectedIds.size > 0 && (
              <div className="flex items-center gap-1">
                <span className="text-xs text-muted-foreground">已选 {selectedIds.size} 条</span>
                <Button size="sm" variant="outline" onClick={() => void applyBulk("active")}>
                  启用
                </Button>
                <Button size="sm" variant="outline" onClick={() => void applyBulk("disabled")}>
                  停用
                </Button>
                <Button size="sm" variant="destructive" onClick={() => void applyBulk("delete")}>
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

        <DialogBody className="flex flex-col gap-3 p-3 text-sm">
          {isLoading ? (
            <div className="py-8 text-center text-muted-foreground">加载凭据…</div>
          ) : items.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              {debouncedQ || statusFilter !== "all" ? "无匹配凭据" : "暂无凭据，点击「添加」导入"}
            </div>
          ) : (
            <>
              <div className={`hidden sm:block overflow-x-auto rounded-md border ${isFetching ? "opacity-70" : ""}`}>
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
                    {items.map((cred) => (
                      <TableRow key={cred.ID}>
                        <TableCell>
                          <Checkbox
                            checked={selectedIds.has(cred.ID)}
                            onCheckedChange={(checked) => toggleSelected(cred.ID, checked === true)}
                            aria-label={`选择凭据 ${cred.ID}`}
                          />
                        </TableCell>
                        <TableCell className="font-mono text-xs" title={cred.KeyMasked}>
                          {cred.KeyMasked}
                        </TableCell>
                        <TableCell>{renderStatusBadges(cred.Status, cred.CooldownUntil)}</TableCell>
                        <TableCell
                          className="max-w-[140px] truncate text-xs text-muted-foreground"
                          title={cred.Note}
                        >
                          {cred.Note || "-"}
                        </TableCell>
                        <TableCell className="text-xs">{cred.FailCount}</TableCell>
                        <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                          {formatCredentialTime(cred.LastUsedAt)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {cred.TotalRequests} / {cred.TotalErrors}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-xs"
                              onClick={() =>
                                void setCredentialStatus(
                                  cred.ID,
                                  cred.Status === "disabled" ? "active" : "disabled",
                                )
                              }
                            >
                              {cred.Status === "disabled" ? "启用" : "停用"}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                              onClick={() => void handleDeleteCredential(cred.ID)}
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

              <div className={`sm:hidden space-y-2 ${isFetching ? "opacity-70" : ""}`}>
                {items.map((cred) => (
                  <div key={cred.ID} className="flex items-start gap-2 rounded-md border p-2">
                    <Checkbox
                      checked={selectedIds.has(cred.ID)}
                      onCheckedChange={(checked) => toggleSelected(cred.ID, checked === true)}
                      className="mt-0.5 shrink-0"
                      aria-label={`选择凭据 ${cred.ID}`}
                    />
                    <div className="min-w-0 flex-1 space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-[11px] truncate">{cred.KeyMasked}</span>
                        {renderStatusBadges(cred.Status, cred.CooldownUntil)}
                      </div>
                      <p className="text-[10px] text-muted-foreground">
                        {cred.Note || "-"} · 失败 {cred.FailCount} · 最近{" "}
                        {formatCredentialTime(cred.LastUsedAt)}
                      </p>
                      <div className="flex gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-2 text-[11px]"
                          onClick={() =>
                            void setCredentialStatus(
                              cred.ID,
                              cred.Status === "disabled" ? "active" : "disabled",
                            )
                          }
                        >
                          {cred.Status === "disabled" ? "启用" : "停用"}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-2 text-[11px] text-destructive hover:text-destructive"
                          onClick={() => void handleDeleteCredential(cred.ID)}
                        >
                          删除
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </DialogBody>

        <div className="flex-shrink-0 flex flex-wrap items-center justify-between gap-2 border-t px-1 pt-3">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {pool.ReferencedBy === 0 ? (
              <span>尚未被任何分组引用</span>
            ) : (
              <span>
                被{" "}
                <Badge variant="outline" className="text-[10px] align-middle">
                  {pool.ReferencedBy} 个分组
                </Badge>{" "}
                引用
              </span>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span>共 {total} 条</span>
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                variant="outline"
                className="h-7 px-2"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
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
      <ImportCredentialsDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImport={handleImport}
        isImporting={batchImport.isPending}
      />
    </Dialog>
  );
}
