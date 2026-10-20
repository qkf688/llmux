import type { ModelWithProvider } from "@/lib/api";

/**
 * 判断某条关联的协议集合是否仍处于**继承态**（`Protocols === null`）。
 *
 * 为什么需要它：协议检测报告的「一键勾选」写的是显式覆盖，会把继承改成固定设置——
 * 之后供应商层再增删协议，这条关联不再跟着变。当下两条协议可能完全一致，用户在界面上
 * 看不出差别，所以写入前要明确提示这个状态变化。
 *
 * 找不到该关联（列表还没回流 / 已被删除）时返回 false：此时提示「继承态」可能是错的，
 * 不提示更安全——写入口径不受影响，写入本来就是显式覆盖。
 */
export function isProtocolsInherited(
  associations: ModelWithProvider[],
  associationId: number | null,
): boolean {
  if (associationId === null) return false;
  const association = associations.find((item) => item.ID === associationId);
  if (!association) return false;
  return association.Protocols === null;
}
