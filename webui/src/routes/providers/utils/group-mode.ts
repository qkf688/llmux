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
 * link 模式下拉可选的号池：手建池 + 当前绑定的专属池。
 *
 * 专属池不进选项：它是「专属某分组」的池，拿它当共享池关联会让两个分组共用一批凭据；
 * 而分组模式不落库（保存只提交 pool_id），下次打开这个供应商时该分组会被推回
 * 「直接添加」，「添加 Key」就把凭据写进了别人的池。
 * 当前绑定的池若本身是专属池（历史绑定 / 用户显式切过模式）仍要留在选项里，
 * 否则下拉显示空白，用户看不出自己绑的是哪个池。
 */
export function linkablePoolOptions(
  pools: PoolOption[],
  current?: PoolOption,
): PoolOption[] {
  const manual = pools.filter((pool) => !pool.autoCreated);
  return current?.autoCreated ? [...manual, current] : manual;
}
