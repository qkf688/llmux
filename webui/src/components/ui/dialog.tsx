import * as React from "react"
import * as DialogPrimitive from "@radix-ui/react-dialog"
import { cva, type VariantProps } from "class-variance-authority"
import { XIcon } from "lucide-react"

import { cn } from "@/lib/utils"

function Dialog({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50",
        className
      )}
      {...props}
    />
  )
}

// 全站 dialog 尺寸配方的单一数据源：页面只声明语义档位，不再自己拼 max-w / max-h / flex。
//
// 宽度一律带 `sm:` 前缀。基类过去写死 `sm:max-w-lg`，而页面传的是无前缀的 `max-w-2xl`——
// 修饰符不同，tailwind-merge 不认为二者冲突故并存，最终桌面端被位置更靠后的断点规则吃掉。
// 全站 8 个 dialog 踩过这个坑，声明宽度和实际宽度长期不一致。
//
// 高度分两类，不是漏配，别顺手给 md 补上固定高度：
//   - lg / xl 固定高度：这两档装列表、详情、测试结果、重管理面板，内容量在弹窗生命周期内
//     会变（切 tab、SSE 流式追加、加载中→加载完）。不固定的话，Radix 用
//     `translate-y-[-50%]` 垂直居中，高度一变整个弹窗就跳位。
//   - sm / md / menu / sheet 只给上限：确认框和表单的内容量从挂载到卸载基本是常量，
//     高度天然不跳，固定它只换来大片留白——正是「占地大内容少」这个抱怨的来源。
//     上限本身可以给得宽松（md 取 720px，与 xl 的固定高度同值）：它只在内容真的长到
//     那个程度时才生效，短表单该多高还是多高，不会因此留白。给紧了反而让长表单
//     （provider-form / association-form 这类十几个字段的）滚动区被压得只剩几行。
//
// md 与 lg 宽度相同（都是 sm:max-w-lg）不是复制粘贴漏改：两档的区别在高度行为，不在宽度。
// 实测 2xl / xl 对列表和详情都偏宽，收到 lg 才合手；而宽度一致恰好让「同一个弹窗从表单档
// 换到列表档」不会横向跳。别因为两档宽度相同就把它们合并——合并会丢掉固定/上限这个分界。

// 用 dvh 而非 vh，避开移动端地址栏伸缩带来的二次跳动。
//
// 无论哪档，Header / Footer 之间的内容都要包一层 DialogBody：上限档顶到上限后同样需要
// 内部滚动，只写 max-h 不给 overflow 等于一条没人执行的声明。
const dialogContentVariants = cva(
  "bg-background data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-50 flex w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] flex-col gap-4 rounded-lg border p-6 shadow-lg duration-200",
  {
    variants: {
      size: {
        sm: "max-h-[85dvh] sm:max-w-md",
        md: "max-h-[min(720px,90dvh)] sm:max-w-lg",
        lg: "h-[min(640px,90dvh)] sm:max-w-lg",
        xl: "h-[min(720px,90dvh)] sm:max-w-3xl",
        menu: "max-h-[80dvh] gap-0 p-0 sm:max-w-[300px]",
        sheet: "max-h-[80dvh] gap-0 p-0 sm:max-w-lg",
      },
    },
    defaultVariants: {
      size: "md",
    },
  }
)

function DialogContent({
  className,
  children,
  size,
  showCloseButton = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> &
  VariantProps<typeof dialogContentVariants> & {
    showCloseButton?: boolean
  }) {
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(dialogContentVariants({ size, className }))}
        {...props}
      >
        {children}
        {showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-4 right-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
          >
            <XIcon />
            <span className="sr-only">Close</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn(
        "flex shrink-0 flex-col gap-2 text-center sm:text-left",
        className
      )}
      {...props}
    />
  )
}

// Header / Footer 之间唯一的滚动容器。外框尺寸由 size 档位定死，溢出全部由这里内部消化，
// 所以 Header / Footer 常驻可见、弹窗整体不重排。
// `min-h-0` 不能省：flex 项默认 min-height:auto，不归零的话它不肯收缩，overflow 永远触发不了。
function DialogBody({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-body"
      className={cn("min-h-0 flex-1 overflow-y-auto", className)}
      {...props}
    />
  )
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex shrink-0 flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className
      )}
      {...props}
    />
  )
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-lg leading-none font-semibold", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-muted-foreground text-sm", className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogBody,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}
 
