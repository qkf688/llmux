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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
  CredentialDeleteDialog,
  type CredentialDeleteTarget,
} from "./credential-delete-dialog";
import { CredentialKeyCell } from "./credential-key-cell";
import { CredentialTestButton } from "./credential-test-button";
import { useCredentialKeyReveal } from "./use-credential-key-reveal";
import { useCredentialTesting } from "./use-credential-testing";
import { CredentialStatusBadge } from "@/components/credentials/credential-status-badge";
import {
  CREDENTIAL_STATUS_DOT_CLS,
  CREDENTIAL_STATUS_LABEL,
  CREDENTIAL_STATUS_ORDER,
  credentialToggleAction,
} from "@/components/credentials/credential-status";
import { CredentialStats } from "./credential-stats-cell";

const PAGE_SIZE = 20;
// 筛选下拉 = 「全部」+ 状态顺序单表派生：状态清单只活在 components/credentials/credential-status 一处
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
  const [deleteTarget, setDeleteTarget] = useState<CredentialDeleteTarget | null>(null);
  const [page, setPage] = useState(1);

  useEffect(() => {
    setSearchText("");
    setDebouncedQ("");
    setStatusFilter("all");
    setSelectedIds(new Set());
    setImportOpen(false);
    setDeleteTarget(null);
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

  // 已显示的明文只在当前视图内有效：翻页 / 筛选 / 搜索 / 关弹窗一律作废（契约见 useCredentialKeyReveal）
  const keyReveal = useCredentialKeyReveal({
    active: open,
    view: `${page}|${statusFilter}|${debouncedQ}`,
  });

  // 「按凭据测试」结论与明文同视图生命周期：翻页/筛选/搜索/关弹窗一律作废（契约见 useCredentialTesting）
  const credentialTesting = useCredentialTesting({
    poolId: open ? poolId : null,
    view: open ? `${page}|${statusFilter}|${debouncedQ}` : "closed",
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
        // 被删凭据的行随即消失，明文必须同时出栈（不等翻页/筛选才作废）
        keyReveal.forget(ids);
        credentialTesting.forget(ids);
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
      // 行消失的同时丢弃其明文：被删的 key 往往正因为已泄露，最不该留在内存里
      keyReveal.forget([id]);
      credentialTesting.forget([id]);
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

  const confirmDelete = () => {
    if (!deleteTarget) return;
    const target = deleteTarget;
    // 确认后先关弹窗，删除动作的 toast 反馈由 handleDelete 负责
    setDeleteTarget(null);
    if (target.kind === "single") {
      void handleDeleteCredential(target.id);
    } else {
      void applyBulk("delete");
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

  const renderStatusBadges = (status: string, cooldownUntil: string | null) => (
    <CredentialStatusBadge status={status} cooldownUntil={cooldownUntil} />
  );

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
            <Select
              value={statusFilter}
              onValueChange={(v) => {
                setStatusFilter(v as "all" | CredentialStatus);
                setPage(1);
              }}
            >
              <SelectTrigger className="w-36" aria-label="按状态筛选">
                <SelectValue placeholder="全部状态" />
              </SelectTrigger>
              <SelectContent align="start" className="min-w-[9rem]">
                {STATUS_FILTER_OPTIONS.map((s) =>
                  s === "all" ? (
                    <SelectItem key={s} value="all">
                      全部状态
                    </SelectItem>
                  ) : (
                    <SelectItem key={s} value={s}>
                      <span className={`size-2 rounded-full ${CREDENTIAL_STATUS_DOT_CLS[s]}`} />
                      {CREDENTIAL_STATUS_LABEL[s]}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
            {selectedIds.size > 0 && (
              <div className="flex items-center gap-1">
                <span className="text-xs text-muted-foreground">已选 {selectedIds.size} 条</span>
                <Button size="sm" variant="outline" onClick={() => void applyBulk("active")}>
                  启用
                </Button>
                <Button size="sm" variant="outline" onClick={() => void applyBulk("disabled")}>
                  停用
                </Button>
                <Button size="sm" variant="destructive" onClick={() => setDeleteTarget({ kind: "bulk", count: selectedIds.size })}>
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
                      <TableHead>统计</TableHead>
                      <TableHead className="w-[160px]">操作</TableHead>
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
                        <TableCell>
                          <CredentialKeyCell
                            credId={cred.ID}
                            keyMasked={cred.KeyMasked}
                            reveal={keyReveal}
                          />
                        </TableCell>
                        <TableCell>{renderStatusBadges(cred.Status, cred.CooldownUntil)}</TableCell>
                        <TableCell
                          className="max-w-[140px] truncate text-xs text-muted-foreground"
                          title={cred.Note}
                        >
                          {cred.Note || "-"}
                        </TableCell>
                        <TableCell className="text-xs whitespace-nowrap">
                          <CredentialStats
                            totalRequests={cred.TotalRequests}
                            totalErrors={cred.TotalErrors}
                            lastUsedAt={cred.LastUsedAt}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <CredentialTestButton
                              outcome={credentialTesting.outcomeOf(cred.ID)}
                              keyMasked={cred.KeyMasked}
                              onTest={() => void credentialTesting.run(cred.ID)}
                            />
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-xs"
                              onClick={() =>
                                void setCredentialStatus(
                                  cred.ID,
                                  credentialToggleAction(cred.Status, cred.CooldownUntil).next,
                                )
                              }
                            >
                              {credentialToggleAction(cred.Status, cred.CooldownUntil).label}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                              onClick={() =>
                                setDeleteTarget({ kind: "single", id: cred.ID, keyMasked: cred.KeyMasked })
                              }
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
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                        <CredentialKeyCell
                          credId={cred.ID}
                          keyMasked={cred.KeyMasked}
                          reveal={keyReveal}
                          size="xs"
                        />
                        {renderStatusBadges(cred.Status, cred.CooldownUntil)}
                      </div>
                      <p className="text-[10px] text-muted-foreground">{cred.Note || "-"}</p>
                      <CredentialStats
                        totalRequests={cred.TotalRequests}
                        totalErrors={cred.TotalErrors}
                        lastUsedAt={cred.LastUsedAt}
                        className="text-[10px]"
                      />
                      <div className="flex gap-1">
                        <CredentialTestButton
                          outcome={credentialTesting.outcomeOf(cred.ID)}
                          keyMasked={cred.KeyMasked}
                          onTest={() => void credentialTesting.run(cred.ID)}
                        />
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-2 text-[11px]"
                          onClick={() =>
                            void setCredentialStatus(
                              cred.ID,
                              credentialToggleAction(cred.Status, cred.CooldownUntil).next,
                            )
                          }
                        >
                          {credentialToggleAction(cred.Status, cred.CooldownUntil).label}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-2 text-[11px] text-destructive hover:text-destructive"
                          onClick={() =>
                            setDeleteTarget({ kind: "single", id: cred.ID, keyMasked: cred.KeyMasked })
                          }
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
      <CredentialDeleteDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        target={deleteTarget}
        onConfirm={confirmDelete}
      />
      <ImportCredentialsDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImport={handleImport}
        isImporting={batchImport.isPending}
      />
    </Dialog>
  );
}
