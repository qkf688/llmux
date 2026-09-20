import { beforeEach, describe, expect, it } from "vitest";
import { queryClient } from "@/lib/query-client";
import { authStore } from "./auth-store";

// 回归（Stage 9 / find-issues 2026-09-16）：登出会话失效后，机密域明文（api-keys 全量
// 明文等）不得继续驻留 react-query 缓存——gcTime 5min 内同机 DevTools 可查读。
describe("authStore.clearSession 清 react-query 缓存", () => {
  beforeEach(() => {
    queryClient.clear();
    authStore
      .getState()
      .setSession("token-1", { id: 1, username: "admin", role: "admin" });
  });

  it("清空会话态的同时移除全部 query 缓存，机密明文不跨会话驻留", () => {
    queryClient.setQueryData(["api-keys"], [{ ID: 1, ApiKey: "sk-plaintext" }]);
    queryClient.setQueryData(["providers"], [{ ID: 1, Name: "p" }]);
    expect(queryClient.getQueryCache().getAll()).toHaveLength(2);

    authStore.getState().clearSession();

    // 会话态照旧清空（原有行为不回归）
    expect(authStore.getState().token).toBeNull();
    expect(authStore.getState().user).toBeNull();
    // 缓存整体移除：明文键与普通数据都不得残留
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
    expect(queryClient.getQueryData(["api-keys"])).toBeUndefined();
  });

  it("尚未建立任何缓存时清空不抛错（401 失效路径也经此动作）", () => {
    expect(() => authStore.getState().clearSession()).not.toThrow();
  });
});
