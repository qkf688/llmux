import type { HealthCheckStatus } from "@/lib/api";

/**
 * 健康检测三态的文案与配色单源（三态值域本身由 `HealthCheckStatus` 与后端同源）。
 *
 * 为什么收在一处：状态刚从两态扩到三态，散落的 `status === "success"` 判定漏掉
 * not_measured 不会报错，只会把「本机没测成」静默显示成上游失败；展示面按整张
 * Record 取用后，新增第四个状态会让每个面一起编译报错。
 *
 * 放在 lib 而非 health-check-logs 页面内：批量检测结果弹窗（components/ 下）与日志页
 * 同用一份，而 components 不能反向依赖 routes 内部工具。
 */
const HEALTH_CHECK_STATUS_LABELS: Record<HealthCheckStatus, string> = {
  success: "成功",
  error: "失败",
  not_measured: "未完成",
};

/** 纯文字展示面（表格单元格、详情弹窗）的语义色 */
export const HEALTH_CHECK_STATUS_TEXT_CLASS: Record<HealthCheckStatus, string> = {
  success: "text-success",
  error: "text-destructive",
  not_measured: "text-muted-foreground",
};

/** 徽标（带底色）展示面的语义色：未完成刻意低对比，不与上游失败抢同一视觉权重 */
export const HEALTH_CHECK_STATUS_BADGE_CLASS: Record<HealthCheckStatus, string> = {
  success: "bg-success-tint text-success-foreground",
  error: "bg-destructive-tint text-destructive-tint-foreground",
  not_measured: "bg-muted text-muted-foreground",
};

/** 状态清单（筛选下拉按此顺序铺开）：从文案表派生，新增状态不会漏进下拉 */
export const HEALTH_CHECK_STATUSES = Object.keys(HEALTH_CHECK_STATUS_LABELS) as HealthCheckStatus[];

export function getHealthCheckStatusLabel(status: HealthCheckStatus): string {
  return HEALTH_CHECK_STATUS_LABELS[status];
}
