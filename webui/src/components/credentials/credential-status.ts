/**
 * 凭据状态展示契约（label / 色板 / 冷却判定）——号池页与供应商编辑弹窗共用。
 *
 * 为什么从 components/number-pools 移到 components/credentials：凭据提为第一等资源后
 * （`/api/credentials`），供应商分组的内联 key 行也要显示同一套状态（判停后可一键恢复），
 * 状态词汇与配色必须只有一处来源，否则两个页面会漂移出两套说法。
 * 徽标组件在 `credential-status-badge.tsx`（同目录），本文件只放常量与纯函数
 * （eslint react-refresh 要求组件文件不混出常量）。
 *
 * 冷却不占状态机——UI 用 CooldownUntil 派生 badge，不在此表。
 */
import type { CredentialStatus } from "@/lib/api";

export const CREDENTIAL_STATUS_LABEL: Record<CredentialStatus, string> = {
  active: "健康",
  disabled: "停用",
  error: "错误",
  temp_unsched: "临时停调度",
};

/** 列表健康概览与筛选下拉共用的状态顺序 */
export const CREDENTIAL_STATUS_ORDER: CredentialStatus[] = [
  "active",
  "error",
  "disabled",
  "temp_unsched",
];

export const CREDENTIAL_STATUS_DOT_CLS: Record<CredentialStatus, string> = {
  active: "bg-success",
  error: "bg-destructive",
  disabled: "bg-muted-foreground/40",
  temp_unsched: "bg-warning",
};

export const CREDENTIAL_STATUS_BADGE_CLS: Record<CredentialStatus, string> = {
  active: "bg-success-tint text-success-foreground",
  error: "bg-destructive-tint text-destructive-tint-foreground",
  disabled: "bg-muted text-muted-foreground",
  temp_unsched: "bg-warning-tint text-warning-foreground",
};

/** 冷却 badge（派生自 CooldownUntil，非存储状态） */
export const COOLDOWN_BADGE_CLS = "bg-warning-tint text-warning-foreground";

export function isCredentialInCooldown(cooldownUntil: string | null | undefined): boolean {
  if (!cooldownUntil) return false;
  const t = Date.parse(cooldownUntil);
  return !Number.isNaN(t) && t > Date.now();
}

/**
 * 启停按钮的唯一判据：按「现在能不能用」决定显示什么、点下去切到哪个状态。
 *
 * 为什么不按「是不是人工关的」判：系统判停与冷却中的凭据本来就归人管，
 * 页面上给它们写「停用」会把一条还能自己恢复的东西钉成人工终态（后台不再管）。
 * 用户点了「启用」就是要它立刻能用，所以切回 active 一律走 models 那侧的
 * 恢复定义（连带清失败计数与冷却窗口），本页只管显示与去向。
 *
 * 空状态串按可用处理，与后端选路 `service/channel/credential_select.go`
 * 「status 非 active 才剔除」一致——否则空串会被本页说成不可用而后端照用。
 */
export function credentialToggleAction(
  status: string,
  cooldownUntil: string | null | undefined,
): { label: "启用" | "停用"; next: CredentialStatus } {
  const usable = (status === "" || status === "active") && !isCredentialInCooldown(cooldownUntil);
  return usable ? { label: "停用", next: "disabled" } : { label: "启用", next: "active" };
}

export function formatCredentialLastUsed(iso: string | null | undefined): string {
  // 统计格专用相对时间：null = 从未成功用过（坏 key 停格可读），紧凑无空格。
  // 与旧 formatCredentialTime 的区别只在 null/非法语义与空格——旧函数无消费方后删除，
  // 避免两套时间口径并存漂移。
  if (!iso) return "从未使用";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "从未使用";
  const diffSec = Math.round((Date.now() - t) / 1000);
  if (diffSec < 60) return "刚刚";
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}小时前`;
  if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)}天前`;
  return new Date(t).toLocaleDateString();
}