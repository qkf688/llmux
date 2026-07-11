import type { ComponentType, LazyExoticComponent } from "react";
import { lazy } from "react";

export type AppRouteLayout = "none" | "app";

export type AppRouteNav = {
  label: string;
  /** 侧栏排序，越小越靠前 */
  order: number;
};

export type AppRouteConfig = {
  /** 完整 path：login 为 /login；app 内为 /providers 或 logs/:logId/chat-io */
  path: string;
  lazy: () => Promise<{ default: ComponentType }>;
  layout: AppRouteLayout;
  /** Layout 下的 index 路由（path 为 /） */
  index?: boolean;
  /** 有 nav 的项进入侧栏；login / chat-io 等不设 */
  nav?: AppRouteNav;
};

/**
 * 路由单一数据源。
 * 新增页面：加配置项 + 新建页面文件即可，不必改 App 内手写 Route 列表。
 */
export const appRoutes: AppRouteConfig[] = [
  {
    path: "/login",
    layout: "none",
    lazy: () => import("./login"),
  },
  {
    path: "/",
    layout: "app",
    index: true,
    lazy: () => import("./home"),
    nav: { label: "首页", order: 10 },
  },
  {
    path: "/providers",
    layout: "app",
    lazy: () => import("./providers"),
    nav: { label: "提供商管理", order: 20 },
  },
  {
    path: "/models",
    layout: "app",
    lazy: () => import("./models"),
    nav: { label: "模型管理", order: 30 },
  },
  {
    path: "/virtual-models",
    layout: "app",
    lazy: () => import("./virtual-models"),
    nav: { label: "虚拟模型", order: 40 },
  },
  {
    path: "/model-providers",
    layout: "app",
    lazy: () => import("./model-providers"),
    nav: { label: "模型提供商关联", order: 50 },
  },
  {
    path: "/logs",
    layout: "app",
    lazy: () => import("./logs"),
    nav: { label: "请求日志", order: 60 },
  },
  {
    path: "/logs/:logId/chat-io",
    layout: "app",
    lazy: () => import("./log-chat"),
  },
  {
    path: "/health-check-logs",
    layout: "app",
    lazy: () => import("./health-check-logs"),
    nav: { label: "健康检测日志", order: 70 },
  },
  {
    path: "/model-sync-logs",
    layout: "app",
    lazy: () => import("./model-sync-logs"),
    nav: { label: "模型同步日志", order: 80 },
  },
  {
    path: "/database",
    layout: "app",
    lazy: () => import("./database"),
    nav: { label: "数据库管理", order: 90 },
  },
  {
    path: "/settings",
    layout: "app",
    lazy: () => import("./settings"),
    nav: { label: "系统设置", order: 100 },
  },
];

/** Layout 壳本身的 lazy（不计入 path 对照表） */
export const layoutLazy = () => import("./layout");

export type LazyPage = LazyExoticComponent<ComponentType>;

/** 将 config 的 lazy 转为 React.lazy 组件（每个 path 只创建一次） */
export function createLazyPages(
  routes: AppRouteConfig[] = appRoutes,
): Map<string, LazyPage> {
  const map = new Map<string, LazyPage>();
  for (const route of routes) {
    map.set(route.path, lazy(route.lazy));
  }
  return map;
}

/** 侧栏：带 nav 的配置，按 order 排序 */
export function getNavRoutes(routes: AppRouteConfig[] = appRoutes) {
  return routes
    .filter((r): r is AppRouteConfig & { nav: AppRouteNav } => r.nav != null)
    .slice()
    .sort((a, b) => a.nav.order - b.nav.order);
}

/** app 布局下的子路由 path（去掉前导 /，index 用 undefined） */
export function appChildPath(route: AppRouteConfig): string | undefined {
  if (route.index) return undefined;
  return route.path.replace(/^\//, "");
}