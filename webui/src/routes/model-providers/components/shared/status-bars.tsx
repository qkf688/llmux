import type { HealthCheckStatus } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

// 格子取值有两种来源：布尔来自 /model-providers/status（聊天日志成败，仍是两态），
// 三态字符串来自 /model-providers/health-status（成功 / 失败 / 未完成）。
type StatusBarValue = boolean | HealthCheckStatus;

type StatusBarsProps = {
  bars: StatusBarValue[] | undefined;
  successClassName: string;
  failClassName: string;
  notMeasuredClassName?: string;
  barClassName?: string;
  emptyTextClassName?: string;
  showTitle?: boolean;
  successTitle?: string;
  failTitle?: string;
  notMeasuredTitle?: string;
};

function toStatusKind(value: StatusBarValue): HealthCheckStatus {
  if (value === "not_measured") return "not_measured";
  if (value === "success" || value === true) return "success";
  return "error";
}

export function StatusBars({
  bars,
  successClassName,
  failClassName,
  notMeasuredClassName = "bg-muted-foreground/40",
  barClassName = "w-1 h-6",
  emptyTextClassName = "text-xs text-muted-foreground",
  showTitle = false,
  successTitle = "成功",
  failTitle = "失败",
  notMeasuredTitle = "未完成（本机原因）",
}: StatusBarsProps) {
  if (!bars) {
    return <Spinner />;
  }

  if (bars.length === 0) {
    return <div className={emptyTextClassName}>无数据</div>;
  }

  return (
    <div className="flex space-x-1 items-end h-6">
      {bars.map((value, index) => {
        const kind = toStatusKind(value);
        // 未完成照旧占一格：跳过会让格子数与检测次数错位，拿旧数据补位等于把本机问题记成上游结论
        const colorClassName =
          kind === "success"
            ? successClassName
            : kind === "not_measured"
              ? notMeasuredClassName
              : failClassName;
        const title =
          kind === "success" ? successTitle : kind === "not_measured" ? notMeasuredTitle : failTitle;

        return (
          <div
            key={index}
            className={`${barClassName} ${colorClassName}`}
            title={showTitle ? title : undefined}
          />
        );
      })}
    </div>
  );
}
