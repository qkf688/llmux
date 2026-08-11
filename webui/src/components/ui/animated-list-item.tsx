import * as React from "react";
import { motion } from "motion/react";
import { DURATION, EASING, LIST_ITEM_VARIANTS } from "@/lib/animations/fluid-transitions";
import { useMotionFallback } from "@/components/ui/use-motion-fallback";

type AnimatedListItemProps = {
  /** 列表项自身样式；transform 类的 hover 微交互请放在内层元素上，见下方分层说明 */
  className?: string;
  children: React.ReactNode;
  /**
   * 置 true 时不自设 initial/animate，改为继承父级 `<StaggerList>` 的 variant 状态，
   * 从而参与 staggerChildren 编排首屏错峰入场（motion 的 variant 传播会在自设 animate 的子级处终止，
   * 所以不置 true 时本组件自驱入场、不受父级 stagger 影响）。exit 不受影响，增删动画照常。
   */
  staggered?: boolean;
};

/**
 * 列表项增删动画载体：新增淡入下滑、删除淡出右移收起、相邻项 layout 平滑位移。
 *
 * 使用约定：
 * - 父容器需包 `<AnimatePresence>`，且 AnimatePresence **不能放进条件分支**
 *   （否则删到空列表时它会随分支切换整体卸载，正在 exit 的 item 被强制拔掉，动画不播）。
 *   不需要首屏入场时给 AnimatePresence 加 `initial={false}`；需要首屏错峰入场则改为
 *   外层包 `<StaggerList>` + 本组件传 `staggered`，两者不要混用。
 * - 本组件是 motion 载体，**transform 类的 hover 微交互（如 hover-lift）必须挂在内层元素**：
 *   motion 会在本元素留内联 transform，覆盖 CSS 的 `:hover` transform（同 home.tsx 的既有坑）。
 *   背景色 / 边框色类（如 `hover:bg-muted/50`）不涉及 transform，挂在本组件上无冲突。
 * - key 必须稳定（后端 ID 或唯一业务标识），AnimatePresence 靠 key 追踪进出。
 */
export function AnimatedListItem({ className, children, staggered = false }: AnimatedListItemProps) {
  // reduced-motion 退化成普通 div：index.css 的 prefers-reduced-motion 只归零 CSS 动画，
  // 拦不住 motion 的 JS 动画；不挂 motion 也顺带避免残留内联 transform。
  const fallback = useMotionFallback(className, children);
  if (fallback) return fallback;

  return (
    <motion.div
      layout
      variants={LIST_ITEM_VARIANTS}
      // staggered 时省略 initial/animate，让父级 StaggerList 的 staggerChildren 接管入场编排
      {...(staggered ? {} : { initial: "initial", animate: "animate" })}
      exit="exit"
      // 进出场时长在 variants 内声明；这里只配 layout（相邻项位移）的过渡
      transition={{ layout: { duration: DURATION.listLayout, ease: EASING.easeOutExpo } }}
      className={className}
    >
      {children}
    </motion.div>
  );
}
