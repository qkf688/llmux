import type { ReactNode } from "react";
import { Search } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";

type PageToolbarProps = {
  /**
   * 工具栏内容：筛选项（ToolbarFilter）、搜索框（ToolbarSearch）、操作按钮组，按需顺序排列。
   * 全部作为同一个 flex 容器的直接子元素——不再按 slot 各包一层 wrapper，
   * 否则窄屏隐藏内容时空 wrapper 仍占一个 flex 项，被 gap 撑出多余间距。
   */
  children: ReactNode;
  /** 覆写外壳布局类；卡片配方本身不接受覆写 */
  className?: string;
};

/**
 * 页头之下、列表之上那条工具栏的统一卡片外壳：圆角、边框、底色、阴影、内距只在此处定义一次。
 * 各页面禁止再手写 `rounded-xl border bg-card ... shadow-sm` 配方（DRY 单一数据源，与 PageHeader / TableCard 三分内距）。
 * 布局用 flex-wrap 而非 grid 列数：增减筛选项时调用方零改动（OCP）。
 */
export function PageToolbar({ children, className }: PageToolbarProps) {
  return (
    <div
      className={cn(
        // items-end 让「带 Label 的筛选项」与「无 Label 的搜索框 / 按钮」底部对齐
        // 内距与间距对齐基准 HTML 的 .card.toolbar（padding 10px 12px / gap 10px）
        "flex flex-wrap items-end gap-2.5 rounded-xl border bg-card px-3 py-2.5 shadow-sm flex-shrink-0",
        className,
      )}
    >
      {children}
    </div>
  );
}

type ToolbarFilterProps = {
  /**
   * 控件上方的小标签：多筛选项并排时必传，否则分不清哪个下拉筛什么。
   * 单个筛选项与搜索框同行时省略——placeholder 已自解释，标签会孤零零悬在下拉上方。
   */
  label?: string;
  value: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
  /** SelectItem 列表 */
  children: ReactNode;
  /** 覆写宽度或显隐（如窄屏隐藏传 hidden sm:flex）；标签与控件配方不接受覆写 */
  className?: string;
};

/**
 * 工具栏内的「小标签 + 下拉」筛选单元（标签可省）。
 * 收口原先 logs / health-check-logs / model-providers 三处逐字复制的同一段 JSX。
 */
export function ToolbarFilter({
  label,
  value,
  onValueChange,
  placeholder,
  children,
  className,
}: ToolbarFilterProps) {
  return (
    <div className={cn("flex min-w-[150px] flex-1 flex-col gap-1 text-xs", className)}>
      {label ? (
        <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</Label>
      ) : null}
      <Select value={value} onValueChange={onValueChange}>
        {/* 无标签时用 placeholder 兜无障碍名称，避免只有视觉提示 */}
        <SelectTrigger aria-label={label ?? placeholder} className="h-8 w-full px-2 text-xs">
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>{children}</SelectContent>
      </Select>
    </div>
  );
}

type ToolbarSearchProps = {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  /** 无障碍名称：占位符不足以说明搜索对象时传（如「搜索提供商名称」） */
  ariaLabel?: string;
  /** 覆写宽度或显隐；只作用于外层容器，不会破坏内部图标+输入的排列 */
  className?: string;
};

/**
 * 工具栏内的搜索框：填充式（`bg-muted` + 透明边框，聚焦才浮起为 `bg-card` + 主色边框），
 * 与基准 HTML 的 `.search` 一致——工具栏里的搜索是「弱态输入」，不该和表单输入框一样抢注意力。
 * 视觉由外层容器统一承担，`Input` 只保留行为，故清掉其自带的边框/底色/内距/focus 环。
 *
 * 分两层：外层只承担宽度/显隐（接受 className 覆写），内层固定 flex 排列。
 * 合成一层的话，调用方传 `hidden sm:block` 会被 tailwind-merge 判成 display 冲突而顶掉 `flex`，
 * 图标与输入框错行、文字溢出卡片（已踩过）。
 */
export function ToolbarSearch({ value, onChange, placeholder, ariaLabel, className }: ToolbarSearchProps) {
  return (
    <div className={cn("min-w-[180px] flex-1", className)}>
      <div className="flex h-8 items-center gap-1.5 rounded-lg border border-transparent bg-muted px-2.5 transition-colors focus-within:border-primary focus-within:bg-card">
        <Search className="size-3.5 shrink-0 text-muted-foreground" />
        <Input
          aria-label={ariaLabel}
          placeholder={placeholder}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="h-full border-0 bg-transparent p-0 text-xs shadow-none focus-visible:ring-0 dark:bg-transparent"
        />
      </div>
    </div>
  );
}
