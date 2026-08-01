// 全站统一动效参数（单一来源）。
// 新增动效优先引用这里的 token，禁止在组件里散落魔法缓动值。
// 分工约定：hover 微交互用 CSS transition（见 index.css 的 hover-lift / hover-border 工具类），
// motion 只用于入场动画 / shared layout / 数字滚动。

export const EASING = {
  easeOutExpo: [0.16, 1, 0.3, 1] as const,
} as const;

// 数字滚动等命令式动画的通用时长（ms）
export const NUMBER_ANIMATION_MS = 800;
