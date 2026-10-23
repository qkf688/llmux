import { useMemo } from "react";
import { usePools } from "@/hooks/api";

export type PoolOption = {
  id: number;
  name: string;
  keyCount: number;
  /** 该池是否为供应商分组自动创建的专属池（pools.auto_created） */
  autoCreated: boolean;
};

export type PoolOptions = {
  options: PoolOption[];
  /** 号池列表是否已就绪。列表没回来前不能按「池是不是自动池」推导分组模式——
   *  先猜一个再翻转会让 radio 在用户眼前跳变，故未就绪时由调用方显示未知态。 */
  ready: boolean;
  /** 号池列表请求失败。失败态同样不能推导模式：列表空 ≠ 池被删，
   *  按「不在列表里」推导会把专属池误判成手建池、radio 跟着翻。 */
  error: boolean;
};

/**
 * 号池下拉数据源。表单组件只消费 PoolOption 形状，禁止直接 import 号池页内部实现。
 */
export function usePoolOptions(): PoolOptions {
  const { data: pools = [], isFetched, isError } = usePools();
  return useMemo(
    () => ({
      options: pools.map((p) => ({
        id: p.ID,
        name: p.Name,
        keyCount: p.KeyCount,
        autoCreated: p.AutoCreated,
      })),
      // isFetched 在请求失败后也是 true（TanStack Query v5：dataUpdateCount +
      // errorUpdateCount > 0），必须叠加 !isError 才代表「列表真的拿到了」。
      ready: isFetched && !isError,
      error: isError,
    }),
    [pools, isFetched, isError],
  );
}
