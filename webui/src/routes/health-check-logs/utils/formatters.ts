// 健康检测三态的判定/文案单源在 @/lib/health-check-status（弹窗与日志页共用一份），
// 此处转发成本页统一的导入面：页内组件只从这里取状态语义，禁止自行比较状态字符串。
export {
  HEALTH_CHECK_STATUSES,
  HEALTH_CHECK_STATUS_BADGE_CLASS,
  HEALTH_CHECK_STATUS_TEXT_CLASS,
  getHealthCheckStatusLabel,
} from "@/lib/health-check-status";

export function formatResponseTime(milliseconds: number): string {
  if (milliseconds < 1) {
    return `${(milliseconds * 1000).toFixed(2)} μs`;
  }

  if (milliseconds < 1000) {
    return `${milliseconds.toFixed(2)} ms`;
  }

  return `${(milliseconds / 1000).toFixed(2)} s`;
}
