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

/**
 * 凭据「为什么不可用」的中文说明（后端 CooldownReason 机器码 → 人话）。
 *
 * 为什么不直接显示机器码：后端存的是给程序判断用的码（`auth_fail` / `http_429` /
 * `network` / `http_<状态码>`，见 service/chat 的 cooldownReasonForStatus），管理端
 * 读到的应该是判断依据本身；直接印 `http_503` 只会逼管理员再查一次文档。
 *
 * 为什么收在本文件：这里已是「凭据状态展示契约」的唯一来源（号池详情 + 供应商编辑
 * 弹窗共用），失败原因同属这套词汇；另开文件会让同一件事有两个落点，早晚漂移。
 *
 * 空串返回空串（= 无原因可显示，调用方据此不渲染），不编造「未知原因」占位。
 * 后端**刻意不保存**上游原始报错全文（只留归类），故此处不可能也不该显示原文。
 */
export function describeCredentialFailureReason(reason: string | null | undefined): string {
  const code = (reason ?? "").trim();
  if (!code) return "";

  switch (code) {
    case "auth_fail":
      return "连续鉴权失败（密钥无效或无权限）";
    case "network":
      return "网络错误或超时";
    case "http_429":
      return "上游限流（请求过于频繁）";
  }
  // 后端 cooldownReasonForStatus 产出 http_<状态码>（5xx 直接带码便于排查）。
  if (code.startsWith("http_")) {
    const status = code.slice("http_".length);
    return status ? `上游返回 ${status}` : code;
  }
  // 未登记的新机器码原样露出：宁可难看，也不要藏成「未知」而丢掉排查线索。
  return code;
}

export function formatCredentialLastUsed(iso: string | null | undefined): string {
  // 统计格专用相对时间：null = 从未成功用过（坏 key 停格可读），紧凑无空格。
  // 与旧 formatCredentialTime 的区别只在 null/非法语义与空格——旧函数无消费方后删除，
  // 避免两套时间口径并存漂移。
  return formatRelativeTime(iso, "从未使用");
}

/**
 * 相对时间格式化内核：两个时间口径（上次成功使用 / 上次探测）共用同一份时间算式，
 * 只有「没有值」的措辞不同。
 *
 * 为什么抽内核：两个口径的 null/非法值文案不同（「从未使用」vs「从未探测」），
 * 各自写一份算式迟早漂移（某天改了「天前」的取整方式，另一处没跟着改）。
 */
export function formatRelativeTime(iso: string | null | undefined, neverLabel: string): string {
  if (!iso) return neverLabel;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return neverLabel;
  const diffSec = Math.round((Date.now() - t) / 1000);
  if (diffSec < 60) return "刚刚";
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}小时前`;
  if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)}天前`;
  return new Date(t).toLocaleDateString();
}