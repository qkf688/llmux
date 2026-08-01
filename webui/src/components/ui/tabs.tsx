import * as React from "react"
import * as TabsPrimitive from "@radix-ui/react-tabs"

import { cn } from "@/lib/utils"

const Tabs = TabsPrimitive.Root

// 轻量滑动高亮：测量激活项位置渲染绝对定位高亮层，用 CSS 过渡 left/width 实现滑动，
// 不引入 motion 的 layoutId（避免跨组件冲突）。
// 激活态变化同时监听 ResizeObserver（尺寸）与 MutationObserver（data-state），
// 覆盖非受控 Tabs（Radix 切 data-state 但 TabsList 不重渲染的场景）。
const TabsList = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, children, ...props }, ref) => {
  const listRef = React.useRef<HTMLDivElement | null>(null)
  const [highlight, setHighlight] = React.useState<{ left: number; width: number } | null>(null)

  // 稳定 ref：合并内部 listRef 与外部 ref，避免每渲染重建导致 ref 拆装
  const setListRef = React.useCallback((node: HTMLDivElement | null) => {
    listRef.current = node
    if (typeof ref === "function") ref(node)
    else if (ref) ref.current = node
  }, [ref])

  // useLayoutEffect 在 paint 前同步测量，避免首帧高亮为 null 导致的闪烁；
  // MutationObserver 兜底 Radix 异步设置 data-state 的非受控场景。
  React.useLayoutEffect(() => {
    const list = listRef.current
    if (!list) return

    const updateHighlight = () => {
      const active = list.querySelector<HTMLElement>('[data-state="active"]')
      if (!active) return
      setHighlight({ left: active.offsetLeft, width: active.offsetWidth })
    }

    updateHighlight()
    // 尺寸变化（窗口缩放、trigger 数量变化）
    const resizeObserver = new ResizeObserver(updateHighlight)
    resizeObserver.observe(list)
    // 激活态切换（覆盖非受控 Tabs：Radix 切 data-state 但 TabsList 不重渲染）
    const mutationObserver = new MutationObserver(updateHighlight)
    mutationObserver.observe(list, {
      subtree: true,
      attributes: true,
      attributeFilter: ["data-state"],
    })
    return () => {
      resizeObserver.disconnect()
      mutationObserver.disconnect()
    }
  }, [])

  return (
    <TabsPrimitive.List
      ref={setListRef}
      className={cn(
        "relative inline-flex h-10 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground",
        className
      )}
      {...props}
    >
      {highlight && (
        <div
          data-slot="tabs-highlight"
          className="absolute inset-y-1 z-0 rounded-md bg-background shadow-sm transition-[left,width] duration-200 ease-out"
          style={{ left: highlight.left, width: highlight.width }}
        />
      )}
      {children}
    </TabsPrimitive.List>
  )
})
TabsList.displayName = TabsPrimitive.List.displayName

const TabsTrigger = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Trigger
    ref={ref}
    className={cn(
      "relative z-10 inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-150 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:text-foreground data-[state=inactive]:hover:text-foreground",
      className
    )}
    {...props}
  />
))
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName

const TabsContent = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Content
    ref={ref}
    className={cn(
      "mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
      className
    )}
    {...props}
  />
))
TabsContent.displayName = TabsPrimitive.Content.displayName

export { Tabs, TabsList, TabsTrigger, TabsContent }
