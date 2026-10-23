import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import "@testing-library/jest-dom/vitest";

// jsdom 没有 ResizeObserver：Radix 的 Select（react-use-size）一挂载就调用它，
// 缺了整棵子树直接抛 "ResizeObserver is not defined"。
// 这里补最小替身——本仓组件不依赖真实尺寸，回调不触发即可。
class ResizeObserverStub implements ResizeObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
globalThis.ResizeObserver ??= ResizeObserverStub;

// vitest 默认 globals: false，全局 afterEach 不存在，RTL 的 auto-cleanup 不会自动注册；
// 显式注册避免 renderHook 挂载的组件跨用例泄漏
afterEach(() => {
  cleanup();
});
