import * as React from "react";
import { useReducedMotion } from "motion/react";

/**
 * motion 载体的统一 reduced-motion 退化：命中时返回普通 div，未命中返回 null。
 *
 * 为什么需要：`index.css` 的 `prefers-reduced-motion` 媒体查询只归零 CSS 动画，拦不住 motion 的
 * JS 动画，所以每个 motion 组件都要自行退化；退化成不挂 motion 的普通 div 还能顺带避免残留内联 transform。
 *
 * 用法（调用方保持早返回，避免把 motion 元素与退化分支写成两套结构）：
 * ```tsx
 * const fallback = useMotionFallback(className, children);
 * if (fallback) return fallback;
 * ```
 */
export function useMotionFallback(
  className: string | undefined,
  children: React.ReactNode,
): React.ReactElement | null {
  const shouldReduceMotion = useReducedMotion();
  if (!shouldReduceMotion) return null;
  return <div className={className}>{children}</div>;
}
