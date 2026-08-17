import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

type TableCardProps = {
  /** 卡内主体：表格 / 列表 / loading / 空态 */
  children: ReactNode;
  /** 卡内底部（分页器等）。传入时自动补 border-t 分隔线，无需调用方自己写 */
  footer?: ReactNode;
  /** 覆写外壳布局类（如自然高度的卡片传 flex-none）；卡片配方本身不接受覆写 */
  className?: string;
};

/**
 * 列表 / 表格类卡片的统一外壳：圆角、边框、底色、阴影只在此处定义一次。
 * 各页面禁止再手写 `rounded-* border bg-card shadow-sm` 配方（DRY 单一数据源）。
 */
export function TableCard({ children, footer, className }: TableCardProps) {
  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border bg-card shadow-sm",
        className
      )}
    >
      <div className="flex min-h-0 flex-1 flex-col">{children}</div>
      {footer ? <div className="flex-shrink-0 border-t px-4 py-2">{footer}</div> : null}
    </div>
  );
}
