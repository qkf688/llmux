import { useMemo } from "react";
import { usePools } from "@/hooks/api";

export type PoolOption = {
  id: number;
  name: string;
  keyCount: number;
};

/**
 * 号池下拉数据源。表单组件只消费 PoolOption 形状，禁止直接 import 号池页内部实现。
 */
export function usePoolOptions(): PoolOption[] {
  const { data: pools = [] } = usePools();
  return useMemo(
    () => pools.map((p) => ({ id: p.ID, name: p.Name, keyCount: p.KeyCount })),
    [pools],
  );
}
