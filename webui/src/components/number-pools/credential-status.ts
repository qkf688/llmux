/**
 * 凭据状态展示契约（label）。
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

export function formatCredentialTime(iso: string | null | undefined): string {
  if (!iso) return "-";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "-";
  const diffSec = Math.round((Date.now() - t) / 1000);
  if (diffSec < 60) return "刚刚";
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`;
  if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)} 天前`;
  return new Date(t).toLocaleString();
}
