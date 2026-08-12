import * as React from "react"

import { cn } from "@/lib/utils"

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div
      data-slot="table-container"
      className="relative w-full"
    >
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-[13px]", className)}
        {...props}
      />
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "bg-muted/50 border-t font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "hover:bg-muted/50 data-[state=selected]:bg-muted border-b border-border/55",
        "transition-colors",
        // hover 时首列内嵌左竖线，标记当前行
        "[&:hover>td:first-child]:shadow-[inset_3px_0_0_var(--color-primary)]",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "text-muted-foreground h-11 px-3.5 text-left align-middle",
        "text-xs font-semibold uppercase tracking-wider whitespace-nowrap",
        "[&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "py-3 px-3.5 align-middle whitespace-nowrap",
        "[&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("text-muted-foreground mt-4 text-sm", className)}
      {...props}
    />
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
}

// sticky 表头公用类：各表格只需额外叠加自己的 z-index（层叠上下文各页不同，故不写进常量）。
export const STICKY_HEADER_CLS =
  "sticky top-0 bg-[var(--table-head-bg)] text-muted-foreground"

// 操作列钉右 + 左侧分隔线：横向滚动时始终可见。
// 背景必须不透明（--table-* 是混在 card 上的实色），否则底层单元格文字会透出来。
// 分隔线用伪元素而非 border-l / box-shadow：Tailwind preflight 给 table 设了
// border-collapse: collapse，该模式下 border 由 table 合并绘制、sticky 单元格边框会被
// 相邻单元格吃掉；box-shadow 画在盒外同样会被盖住。伪元素定位在 sticky 盒内，不受影响。
// 注意：不能加 relative——它和 sticky 同为 position 属性，会覆盖掉 sticky 导致钉右失效；
// sticky 本身已是定位元素，足以作为 absolute 伪元素的参照。
export const STICKY_ACTIONS_DIVIDER =
  "before:absolute before:inset-y-0 before:left-0 before:w-px before:bg-border before:content-['']"
export const STICKY_ACTIONS_HEAD_CLS =
  `sticky right-0 bg-[var(--table-head-bg)] ${STICKY_ACTIONS_DIVIDER}`
export const STICKY_ACTIONS_CELL_CLS =
  `sticky right-0 bg-card group-hover:bg-[var(--table-row-hover-bg)] ${STICKY_ACTIONS_DIVIDER}`

