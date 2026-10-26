import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

// 回归背景：清除关联 / 删除提供商只失效了 providerKeys，没有失效 modelProviderKeys。
// 「模型关联」页读的是另一套 key，而全局 staleTime=60s、refetchOnWindowFocus=false，
// 于是清完再进关联页仍看到已删除的行——用户会以为没清掉并重复点击，
// 第二次得到 deleted: 0（现网现象）。本文件锁「跨缓存族都要失效」这条契约。
const apiMocks = vi.hoisted(() => ({
  getProviders: vi.fn(),
  getProviderModelCatalog: vi.fn(),
  getProviderTemplates: vi.fn(),
  createProvider: vi.fn(),
  updateProvider: vi.fn(),
  deleteProvider: vi.fn(),
  clearProviderAssociations: vi.fn(),
  syncAllProviderModels: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getHealthCheckSettings: vi.fn(),
  updateHealthCheckSettings: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

import { modelProviderKeys } from "@/hooks/api/use-model-providers";
import {
  providerKeys,
  useClearProviderAssociations,
  useDeleteProvider,
} from "./use-providers";

function createClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function makeWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useClearProviderAssociations", () => {
  it("清除后同时失效提供商列表与模型关联列表（两个缓存族）", async () => {
    apiMocks.clearProviderAssociations.mockResolvedValue({
      provider_id: 17,
      provider_name: "zcode",
      deleted: 2,
    });

    const queryClient = createClient();
    // 落一份关联页缓存：不失效的话它会一直带着已删除的行返回给页面
    const assocKey = modelProviderKeys.list(9);
    queryClient.setQueryData(assocKey, [{ ID: 71 }, { ID: 77 }]);
    // 提供商页缓存同理，用来验证原有的失效行为没被顺手改掉
    const providerKey = providerKeys.list();
    queryClient.setQueryData(providerKey, [{ ID: 17 }]);
    // 无 observer 的 query 只会被标记 invalidated（不会立即 refetch），
    // 正是「切页前缓存仍陈旧」的成因，断言就落在这个标记上。
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(false);

    const { result } = renderHook(() => useClearProviderAssociations(), {
      wrapper: makeWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(17);
    });

    await waitFor(() => {
      expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
    });
    // 原有的提供商列表失效不得被覆盖掉
    expect(queryClient.getQueryState(providerKey)?.isInvalidated).toBe(true);
  });
});

describe("useDeleteProvider", () => {
  it("删除后同时失效提供商列表与模型关联列表", async () => {
    apiMocks.deleteProvider.mockResolvedValue(undefined);

    const queryClient = createClient();
    const assocKey = modelProviderKeys.list(9);
    queryClient.setQueryData(assocKey, [{ ID: 77 }]);
    const providerKey = providerKeys.list();
    queryClient.setQueryData(providerKey, [{ ID: 17 }]);
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(false);

    const { result } = renderHook(() => useDeleteProvider(), {
      wrapper: makeWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(17);
    });

    await waitFor(() => {
      expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
    });
    expect(queryClient.getQueryState(providerKey)?.isInvalidated).toBe(true);
  });
});
