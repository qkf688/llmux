import { useMemo } from "react";
import { mockPools } from "@/routes/number-pools/mock/data";

export type PoolOption = {
  id: number;
  name: string;
  keyCount: number;
};

/**
 * 号池下拉数据源（S0 原型挂 mock）。
 * S6 正式实现时替换内部实现为 lib/api 调用（TanStack Query），表单组件零改动；
 * 禁止其它位置直接 import number-pools 的 mock 数据。
 */
export function usePoolOptions(): PoolOption[] {
  return useMemo(
    () => mockPools.map((p) => ({ id: p.id, name: p.name, keyCount: p.credentials.length })),
    [],
  );
}