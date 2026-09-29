import type { HealthCheckLog } from "@/lib/api";

/**
 * 健康检测日志的测试夹具（三态展示用例共用）。
 *
 * 为什么放这里：两个消费方分属 `components/`（批量检测结果弹窗）与
 * `routes/health-check-logs/`（日志列表），route 内的 `test-fixtures.ts` 二者都够不着，
 * 复制一份会在重复块检测里立刻显形。
 */
export function createHealthCheckLog(
  ID: number,
  status: HealthCheckLog["status"],
): HealthCheckLog {
  return {
    ID,
    CreatedAt: "2026-01-01T00:00:00Z",
    model_provider_id: ID,
    model_name: "gpt-test",
    provider_name: "prov",
    provider_model: "gpt-4o",
    status,
    error: "",
    response_time: 12,
    checked_at: "2026-01-01T00:00:00Z",
  };
}
