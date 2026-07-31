import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import "@testing-library/jest-dom/vitest";

// vitest 默认 globals: false，全局 afterEach 不存在，RTL 的 auto-cleanup 不会自动注册；
// 显式注册避免 renderHook 挂载的组件跨用例泄漏
afterEach(() => {
  cleanup();
});
