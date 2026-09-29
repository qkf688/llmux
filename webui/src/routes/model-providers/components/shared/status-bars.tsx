import type { HealthCheckStatus } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

// 只认三态词汇（Stage 36）：这里曾是 `boolean | HealthCheckStatus` 的联合，
// 于是「聊天成败」与「健康检测」两组数据可以互换着传，把前者塞进健康槽会静默编译通过。
// 布尔来自 /model-providers/status 的那一路，由调用方在传参处显式声明映射。
type StatusBarsProps = {
  bars: HealthCheckStatus[] | undefined;
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
      {bars.map((status, index) => {
        // 未完成照旧占一格：跳过会让格子数与检测次数错位，拿旧数据补位等于把本机问题记成上游结论
        const colorClassName =
          status === "success"
            ? successClassName
            : status === "not_measured"
              ? notMeasuredClassName
              : failClassName;
        const title =
          status === "success" ? successTitle : status === "not_measured" ? notMeasuredTitle : failTitle;

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
