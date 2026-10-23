import type { ProviderGroupMode } from "../form-schema";
import type { PoolOption } from "../hooks/use-pool-options";

/**
 * 分组取 Key 的模式解析：用户显式点选优先；没点选时按「当前绑定的号池是不是专属池」推导
 * （专属池 → 直接添加；手建池 → 关联号池）。
 *
 * 返回 null = 未知（号池列表还没回来，或请求失败）。此时**不猜**：先渲染占位，等列表到了再定模式。
 * 猜错的后果是 radio 在用户眼前跳变（先「直接添加」再翻成「关联号池」），比多等 100ms 难受得多。
 */
export function resolveGroupMode(
  poolId: string,
  pool: PoolOption | undefined,
  poolsReady: boolean,
): ProviderGroupMode | null {
  if (!poolId) return "direct";
  if (!poolsReady) return null;
  // 号池不在列表里（已删除）→ 退回关联模式，让用户重选一个存在的池
  if (!pool) return "link";
  return pool.autoCreated ? "direct" : "link";
}

/**
 * link 模式下拉可选的号池：**只有手建池**。
 *
 * 专属池一律不进选项（当前绑定的那个也不例外）：专属池是「专属某分组」的池，
 * 把它当共享池关联会让两个分组共用一批凭据。旧版会把「当前绑定的专属池」留在选项里，
 * 好处是下拉不空、用户看得出自己绑了谁；代价是在没有手建池的库里，下拉里只剩它自己，
 * 看着像「专属池可以关联」——实际是死路，用户会以为专属池被共享了。
 * 现在改成：专属池不进选项，界面用占位 + 说明表达「当前未关联手建号池」。
 */
export function linkablePoolOptions(pools: PoolOption[]): PoolOption[] {
  return pools.filter((pool) => !pool.autoCreated);
}
