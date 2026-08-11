// 全站统一动效参数（单一来源）。
// 新增动效优先引用这里的 token，禁止在组件里散落魔法缓动值。
// 分工约定：hover 微交互用 CSS transition（见 index.css 的 hover-lift / hover-border 工具类），
// motion 只用于入场动画 / shared layout / 数字滚动 / 列表项增删。

import type { Variants } from "motion/react";

export const EASING = {
  easeOutExpo: [0.16, 1, 0.3, 1] as const,
} as const;

// 数字滚动等命令式动画的通用时长（ms）
export const NUMBER_ANIMATION_MS = 800;

// 列表项增删动画时长（秒，motion 的时间单位）。
// 离场比入场短：删除是用户主动操作，反馈要快；入场是结果呈现，可稍缓。
export const DURATION = {
  listEnter: 0.3,
  listExit: 0.24,
  /** 相邻项因增删而位移（layout 动画）的时长 */
  listLayout: 0.3,
} as const;

// 列表项进出场 variants：新增项淡入下滑，删除项淡出右移并收起高度，后续项由 layout 动画平滑上移。
// 高度与内外边距、边框在 exit 归零，配合 overflow hidden 避免收起过程露出内容；
// 否则被删项在 exit 期间仍占位，后续项无法连续上移。
export const LIST_ITEM_VARIANTS: Variants = {
  initial: { opacity: 0, y: -8, scale: 0.98 },
  animate: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: { duration: DURATION.listEnter, ease: EASING.easeOutExpo },
  },
  exit: {
    opacity: 0,
    x: 24,
    scale: 0.96,
    height: 0,
    marginTop: 0,
    marginBottom: 0,
    paddingTop: 0,
    paddingBottom: 0,
    borderWidth: 0,
    overflow: "hidden",
    transition: { duration: DURATION.listExit, ease: EASING.easeOutExpo },
  },
};
