import * as React from "react";
import { motion } from "motion/react";
import {
  STAGGER_CONTAINER_VARIANTS,
  STAGGER_ITEM_VARIANTS,
} from "@/lib/animations/fluid-transitions";
import { useMotionFallback } from "@/components/ui/use-motion-fallback";

type StaggerListProps = {
  className?: string;
  children: React.ReactNode;
  /**
   * 变化时重播整批 stagger 入场（通过 React key 强制重挂载 motion 容器）。
   * 典型用于分页：传入 page 或首项 ID，翻页时列表整体错峰重现。
   * 与 AnimatePresence 增删动画共用时**不要**传：否则每次增删都会整体重播，覆盖单项进出场。
   */
  resetKey?: React.Key;
};

type StaggerItemProps = {
  className?: string;
  children: React.ReactNode;
};

/**
 * 首屏内容 stagger 入场容器：编排直接子项按固定间隔错峰浮现。仅做编排，自身无视觉变化。
 *
 * 子项要求：**不得自设 initial/animate**，否则 motion 的 variant 传播在该子级处终止，
 * 不会参与父级 staggerChildren。适用子项：
 *   - `<StaggerItem>`（本文件导出）—— 直接可用
 *   - `<AnimatedListItem staggered>` —— 传 `staggered` 时省略自身 initial/animate，可作为 stagger 子项
 *
 * 使用约定：
 * - 应在数据就绪后才挂载（如放在页面的 loading 分支之后）：编排靠首次挂载触发 initial→animate，
 *   空列表先挂载再填充会被当作逐项新增而非整批 stagger。
 * - 与 AnimatePresence 组合时，AnimatePresence 需允许首屏入场（不要设 initial={false}），
 *   首屏由本容器编排 stagger，运行时增删仍由 AnimatePresence 逐项处理。
 * - reduced-motion 退化为普通 div。
 */
export function StaggerList({ className, children, resetKey }: StaggerListProps) {
  const fallback = useMotionFallback(className, children);
  if (fallback) return fallback;

  return (
    <motion.div
      key={resetKey}
      className={className}
      variants={STAGGER_CONTAINER_VARIANTS}
      initial="initial"
      animate="animate"
    >
      {children}
    </motion.div>
  );
}

/**
 * stagger 入场单项：淡入 + 上移。入场节奏由父级 StaggerList 的 staggerChildren 编排，
 * 本组件不写 initial/animate（继承父级 variant，参与错峰）。
 *
 * transform 类的 hover 微交互（如 hover-lift）必须挂在内层元素：motion 会在本元素留内联
 * transform，覆盖 CSS 的 :hover transform（同 home.tsx 的既有坑）。
 */
export function StaggerItem({ className, children }: StaggerItemProps) {
  const fallback = useMotionFallback(className, children);
  if (fallback) return fallback;

  return (
    <motion.div className={className} variants={STAGGER_ITEM_VARIANTS}>
      {children}
    </motion.div>
  );
}
