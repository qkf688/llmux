import type { ChatLog } from "@/lib/api";
import { cn } from "@/lib/utils";
import { splitLogSchedule } from "../../utils/log-schedule";

type LogScheduleCellProps = {
  log: ChatLog;
  /** 行文本样式：desktop 默认 text-xs；mobile 传 "truncate text-[11px]" */
  textClassName?: string;
};

/**
 * 调度列（端点层 / 凭据层两行）的共享渲染：desktop 表格与 mobile 列表同构消费，
 * 三态判定与拼串全部委托 splitLogSchedule 单一来源——本组件只做展示，不自带语义。
 */
export function LogScheduleCell({ log, textClassName = "text-xs" }: LogScheduleCellProps) {
  const schedule = splitLogSchedule(log);
  if (!schedule.hasAny) {
    // 空态（S3-3 之前的存量行）：整体单占位符，不渲染两层各自挂 "-"
    return <span className={cn(textClassName, "text-muted-foreground")}>-</span>;
  }
  return (
    <div className="flex flex-col gap-0.5">
      <span className={cn(textClassName, "font-medium")}>{schedule.endpointLine || "-"}</span>
      <span className={cn(textClassName, "text-muted-foreground")}>{schedule.credentialLine || "-"}</span>
    </div>
  );
}
