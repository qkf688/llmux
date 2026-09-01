/**
 * 号池页面总编排：TanStack Query 拉列表；弹窗开闭本地 state。
 * 凭据级操作由详情弹窗内部走 API，不再经 onPoolUpdate 写回。
 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import {
  usePools,
  useCreatePool,
  useUpdatePool,
  useDeletePool,
} from "@/hooks/api";
import type { PoolListItem } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";

export function useNumberPoolsPage() {
  const { data: pools = [], isLoading, isError, error } = usePools();
  const createPool = useCreatePool();
  const updatePool = useUpdatePool();
  const deletePool = useDeletePool();

  const [detailPoolId, setDetailPoolId] = useState<number | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [formPool, setFormPool] = useState<PoolListItem | null>(null);

  // 列表 invalidate 后同步详情弹窗的 KeyCount/StatusCounts/ReferencedBy
  const detailPool =
    detailPoolId == null ? null : (pools.find((p) => p.ID === detailPoolId) ?? null);

  useEffect(() => {
    if (detailOpen && detailPoolId != null && detailPool == null) {
      setDetailOpen(false);
      setDetailPoolId(null);
    }
  }, [detailOpen, detailPoolId, detailPool]);

  const openCreateForm = useCallback(() => {
    setFormPool(null);
    setFormOpen(true);
  }, []);

  const openEditForm = useCallback((pool: PoolListItem) => {
    setFormPool(pool);
    setFormOpen(true);
  }, []);

  const openDetail = useCallback((pool: PoolListItem) => {
    setDetailPoolId(pool.ID);
    setDetailOpen(true);
  }, []);

  const handleDetailOpenChange = useCallback((open: boolean) => {
    setDetailOpen(open);
    if (!open) setDetailPoolId(null);
  }, []);

  const handlePoolSaved = useCallback(
    async (values: { name: string; note?: string }) => {
      try {
        if (formPool) {
          await updatePool.mutateAsync({
            id: formPool.ID,
            data: { name: values.name, note: values.note },
          });
          toast.success("号池已更新");
        } else {
          await createPool.mutateAsync({ name: values.name, note: values.note });
          toast.success("号池已创建");
        }
        setFormOpen(false);
      } catch (err) {
        toast.error(`${formPool ? "更新" : "创建"}失败: ${toErrorMessage(err)}`);
      }
    },
    [formPool, createPool, updatePool],
  );

  const handlePoolDelete = useCallback(
    async (poolId: number) => {
      try {
        await deletePool.mutateAsync(poolId);
        toast.success("号池已删除");
        if (detailPoolId === poolId) {
          setDetailOpen(false);
          setDetailPoolId(null);
        }
      } catch (err) {
        toast.error(`删除失败: ${toErrorMessage(err)}`);
      }
    },
    [deletePool, detailPoolId],
  );

  return {
    pools,
    isLoading,
    isError,
    error,
    detailPool,
    detailOpen,
    setDetailOpen: handleDetailOpenChange,
    formOpen,
    setFormOpen,
    formPool,
    openCreateForm,
    openEditForm,
    openDetail,
    handlePoolSaved,
    handlePoolDelete,
    isSaving: createPool.isPending || updatePool.isPending,
  };
}
