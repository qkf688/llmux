import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// 回归背景：黑名单弹窗写的是 Provider.Blacklisted，而提供商页「关联触发」开关直接读该字段。
// 保存成功后不失效 providerKeys 的话，回到提供商页仍显示旧开关状态（staleTime 60s）。
// 该端点（PUT /providers/blacklist）不触发 autoassoc，故只断 providerKeys、不断关联缓存。
const apiMocks = vi.hoisted(() => ({
  getProviderBlacklist: vi.fn(),
  updateProviderBlacklist: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { providerKeys } from "@/hooks/api/use-providers";
import { modelProviderKeys } from "@/hooks/api/use-model-providers";
import { useModelProvidersBlacklist } from "./use-model-providers-blacklist";

function createClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function makeWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useModelProvidersBlacklist", () => {
  beforeEach(() => {
    apiMocks.getProviderBlacklist.mockReset().mockResolvedValue({ blacklisted_ids: [] });
    apiMocks.updateProviderBlacklist.mockReset().mockResolvedValue({ updated: 2 });
  });

  it("保存黑名单后失效提供商列表缓存（关联触发开关读同一字段）", async () => {
    const queryClient = createClient();
    const providerKey = providerKeys.list();
    const assocKey = modelProviderKeys.list(9);
    queryClient.setQueryData(providerKey, [{ ID: 30, Name: "zcode" }]);
    queryClient.setQueryData(assocKey, [{ ID: 71 }]);
    expect(queryClient.getQueryState(providerKey)?.isInvalidated).toBe(false);

    const { result } = renderHook(
      () =>
        useModelProvidersBlacklist({
          providers: [],
          blacklistDialogOpen: false,
          setBlacklistDialogOpen: vi.fn(),
          blacklistedIds: [30],
          setBlacklistedIds: vi.fn(),
          setBlacklistLoading: vi.fn(),
          setBlacklistSaving: vi.fn(),
          blacklistSearchTerm: "",
          setBlacklistSearchTerm: vi.fn(),
          blacklistFilter: "all",
          setBlacklistFilter: vi.fn(),
        }),
      { wrapper: makeWrapper(queryClient) },
    );

    await act(async () => {
      await result.current.handleSaveBlacklist();
    });

    await waitFor(() => {
      expect(queryClient.getQueryState(providerKey)?.isInvalidated).toBe(true);
    });
    // 该端点不触发 autoassoc：关联缓存不应被牵连失效（避免无谓重取）
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(false);
  });
});
