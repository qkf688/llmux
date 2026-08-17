import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

type StatGridProps = {
  /** StatCard 列表 */
  children: ReactNode;
  /** 覆写列数或间距（如弹窗内想固定 4 列）；卡片配方本身不接受覆写 */
  className?: string;
};

/**
 * 统计卡网格：列数与间距在此定义一次，各页禁止再手写 `grid grid-cols-2 lg:grid-cols-4 gap-*`。
 * 列数取基准 HTML 的 `.stats`（宽屏 4 列 / 窄屏 2 列，gap 10px）——窄屏不降到 1 列，
 * 统计值都很短，单列会把首屏推得太长。
 */
export function StatGrid({ children, className }: StatGridProps) {
  return <div className={cn("grid grid-cols-2 gap-2.5 lg:grid-cols-4", className)}>{children}</div>;
}

/** 圆点语义色：颜色只落在圆点上，数值一律用前景色（与基准 HTML 一致，避免一屏出现多种彩色大字） */
type StatTone = "neutral" | "success" | "danger" | "info";

const dotToneClass: Record<StatTone, string> = {
  neutral: "bg-foreground",
  success: "bg-success",
  danger: "bg-destructive",
  info: "bg-info",
};

type StatCardProps = {
  /** 指标名，如「文件大小」 */
  label: string;
  /** 主数值：已格式化好的字符串或节点，本组件不做格式化 */
  value: ReactNode;
  /** 紧跟数值的小号单位，如 `%`、`页`；与数值同基线，不换行 */
  unit?: ReactNode;
  /** 数值下方的补充说明（如原始字节数）；窄屏隐藏，两列布局下会把卡撑高换行 */
  sub?: ReactNode;
  /** 语义色，只影响 label 前的圆点 */
  tone?: StatTone;
  /** 覆写宽度或跨列（如 col-span-2）；卡片配方与内部排版不接受覆写 */
  className?: string;
};

/**
 * 单指标统计卡：圆点 + 指标名 + 大号数值（+ 单位 / 副说明），供 StatGrid 排布。
 * 卡壳配方（圆角、边框、底色、阴影、内距）只在此处定义一次，与 PageHeader / PageToolbar / TableCard 同族。
 *
 * 外层刻意不带 display 类：调用方传 `hidden` / `col-span-2` 时，tailwind-merge 无从顶掉内部排版
 * （踩过 ToolbarSearch 单层结构被 `sm:block` 顶掉 `flex` 的坑）。
 *
 * 只覆盖「单指标」形态。home 的分组多指标卡（竖排标题栏 + 多行指标）语义不同，不套用本组件，
 * 否则要为它开一堆可选 props，退化成什么都能长的万能卡。
 */
export function StatCard({ label, value, unit, sub, tone = "neutral", className }: StatCardProps) {
  return (
    <div className={cn("rounded-xl border bg-card px-3.5 py-3 shadow-sm", className)}>
      <div className="flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
        <span className={cn("size-[7px] shrink-0 rounded-full", dotToneClass[tone])} />
        <span className="truncate">{label}</span>
      </div>
      <div className="mt-1 text-xl font-semibold tracking-tight tabular-nums">
        {value}
        {unit ? (
          <span className="ml-[3px] text-xs font-medium text-muted-foreground">{unit}</span>
        ) : null}
      </div>
      {sub ? (
        <p className="mt-0.5 hidden truncate text-[11px] text-muted-foreground sm:block">{sub}</p>
      ) : null}
    </div>
  );
}
