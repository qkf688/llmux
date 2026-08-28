/**
 * 号池页面总编排（S0 原型：本地 state 替代 TanStack Query）。
 * 状态：号池列表 / 详情弹窗 / 新建-编辑弹窗；凭据级操作（导入/搜索/筛选/批量）
 * 由详情弹窗内部管理，经 handlePoolUpdate 写回。
 */
import { useCallback, useState } from "react";
import { mockPools } from "../mock/data";
import type { MockPool } from "../types";

export function useNumberPoolsPage() {
  const [pools, setPools] = useState<MockPool[]>(() => mockPools);
  const [detailPool, setDetailPool] = useState<MockPool | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [formPool, setFormPool] = useState<MockPool | null>(null);

  const openCreateForm = useCallback(() => {
    setFormPool(null);
    setFormOpen(true);
  }, []);

  const openEditForm = useCallback((pool: MockPool) => {
    setFormPool(pool);
    setFormOpen(true);
  }, []);

  const openDetail = useCallback((pool: MockPool) => {
    setDetailPool(pool);
    setDetailOpen(true);
  }, []);

  /** 凭据级变更（导入/启停/删除）就地写回列表与详情 */
  const handlePoolUpdate = useCallback((updated: MockPool) => {
    setPools((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
    setDetailPool((prev) => (prev?.id === updated.id ? updated : prev));
  }, []);

  /** 新建/编辑号池保存（S0：本地 id 自增；S2 由后端返回） */
  const handlePoolSaved = useCallback((saved: MockPool) => {
    setPools((prev) => {
      const exists = prev.some((p) => p.id === saved.id);
      if (exists) {
        return prev.map((p) => (p.id === saved.id ? saved : p));
      }
      const nextId = prev.length > 0 ? Math.max(...prev.map((p) => p.id)) + 1 : 1;
      return [...prev, { ...saved, id: nextId }];
    });
    setFormOpen(false);
  }, []);

  const handlePoolDelete = useCallback(
    (poolId: number) => {
      setPools((prev) => prev.filter((p) => p.id !== poolId));
      if (detailPool?.id === poolId) {
        setDetailOpen(false);
        setDetailPool(null);
      }
    },
    [detailPool],
  );

  return {
    pools,
    detailPool,
    detailOpen,
    setDetailOpen,
    formOpen,
    setFormOpen,
    formPool,
    openCreateForm,
    openEditForm,
    openDetail,
    handlePoolUpdate,
    handlePoolSaved,
    handlePoolDelete,
  };
}