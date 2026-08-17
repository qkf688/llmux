import type { ComponentType, ReactNode } from "react";

import { cn } from "@/lib/utils";

interface PageHeaderProps {
  /** 页面主标题 */
  title: string;
  /** 可选副标题：缺省时页头只有一行，会显得过扁，建议各页都给 */
  subtitle?: ReactNode;
  /** 标题左侧图标：只收组件类型，尺寸与配色由本组件统一决定 */
  icon?: ComponentType<{ className?: string }>;
  /** 右侧操作区（按钮等） */
  actions?: ReactNode;
  className?: string;
}

/**
 * 页面级页头：卡片外壳 + 图标 + 标题/副标题两行 + 右侧操作区，供各管理页复用。
 * 统一标签为 h2、字号 text-xl，避免各页 h1/h2 与字号各写一套。
 */
export function PageHeader({ title, subtitle, icon: Icon, actions, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-card px-5 py-4 shadow-sm flex-shrink-0",
        className,
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        {Icon ? (
          <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-foreground">
            <Icon className="size-5" />
          </span>
        ) : null}
        <div className="min-w-0">
          <h2 className="truncate text-xl font-semibold tracking-tight">{title}</h2>
          {subtitle ? (
            <p className="mt-0.5 truncate text-xs text-muted-foreground">{subtitle}</p>
          ) : null}
        </div>
      </div>
      {/* 多按钮页（database 6 个 / logs 4 个）窄屏需换行，故在此统一 flex-wrap，避免各页自己包滚动容器 */}
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
