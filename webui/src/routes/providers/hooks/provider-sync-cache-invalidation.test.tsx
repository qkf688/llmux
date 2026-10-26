import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// 回归背景：同步 / 获取模型会经 modelsync.ActionHooks 在后台改写 model_with_providers
// （自动关联、自动清理）。这些入口只失效了 providerKeys，没失效 modelProviderKeys，
// 于是「模型关联」页在全局 staleTime=60s 内继续显示已失效或缺失的行。
// 本文件锁这条跨缓存族契约。
const apiMocks = vi.hoisted(() => ({
  createProvider: vi.fn(),
  getProviderModelCatalog: vi.fn(),
  getProviderModels: vi.fn(),
  getProviders: vi.fn(),
  syncProviderModels: vi.fn(),
  syncAllProviderModels: vi.fn(),
  updateProvider: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import { modelProviderKeys } from "@/hooks/api/use-model-providers";
import { providerKeys } from "@/hooks/api/use-providers";
import { useProviderSyncActions } from "./use-provider-sync-actions";
import { useAllModelsDialog } from "./use-all-models-dialog";
import { useProviderMutations } from "./use-provider-mutations";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";

function createClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function makeWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

/** 落两份缓存：关联页（必须被失效）+ 提供商目录（原有行为，不得被改掉）。
 *  目录条目要按 ProviderModelCatalog 真实形状构造（Upstream 必填数组）——
 *  用半个对象占位会让被测 hook 的 useMemo 直接抛，红的原因就与契约无关了。 */
function seedCaches(queryClient: QueryClient) {
  const assocKey = modelProviderKeys.list(9);
  const catalogKey = providerKeys.catalog();
  queryClient.setQueryData(assocKey, [{ ID: 71 }, { ID: 77 }]);
  queryClient.setQueryData(catalogKey, [
    { ProviderID: 30, Upstream: [], Custom: [] },
    { ProviderID: 31, Upstream: [], Custom: [] },
  ]);
  expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(false);
  return { assocKey, catalogKey };
}

describe("useProviderSyncActions", () => {
  beforeEach(() => {
    apiMocks.syncAllProviderModels.mockReset().mockResolvedValue({
      logs: [],
      added_total: 0,
      removed_total: 0,
      synced_providers: 0,
      message: "没有检测到模型变化",
    });
  });

  it("全局同步后失效模型关联页缓存（两个缓存族都要刷）", async () => {
    const queryClient = createClient();
    const { assocKey, catalogKey } = seedCaches(queryClient);

    const { result } = renderHook(
      () => useProviderSyncActions({ setSyncingAll: vi.fn() }),
      { wrapper: makeWrapper(queryClient) },
    );

    await act(async () => {
      await result.current.handleSyncAllProviders();
    });

    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
    // 原有的 catalog 失效行为不得被覆盖
    expect(queryClient.getQueryState(catalogKey)?.isInvalidated).toBe(true);
  });
});

describe("useAllModelsDialog", () => {
  beforeEach(() => {
    apiMocks.getProviderModelCatalog.mockReset().mockResolvedValue([
      { ProviderID: 30, Upstream: [], Custom: ["old-model"] },
    ]);
    apiMocks.getProviders.mockReset().mockResolvedValue([]);
    apiMocks.getProviderModels.mockReset().mockResolvedValue([]);
    apiMocks.syncProviderModels.mockReset().mockResolvedValue({
      AddedCount: 1,
      RemovedCount: 0,
    });
    apiMocks.updateProvider.mockReset().mockResolvedValue({});
  });

  const baseArgs = {
    allModelsProvider: { ID: 30, Name: "zcode", Type: "anthropic" } as never,
    setAllModelsProvider: vi.fn(),
    setAllModelsOpen: vi.fn(),
    selectedAllModels: [],
    setSelectedAllModels: vi.fn(),
    customModelInput: "new-model",
    setCustomModelInput: vi.fn(),
    allModelsSearchQuery: "",
    setAllModelsSearchQuery: vi.fn(),
    allModelsTypeFilter: "all" as const,
    setAllModelsTypeFilter: vi.fn(),
    setAllModelsTestResults: vi.fn(),
    setAddingModels: vi.fn(),
    setSyncingModels: vi.fn(),
    autoActionsFlags: { autoAssociateOnAddEnabled: true, autoCleanOnDeleteEnabled: true },
  };

  it("同步上游模型后失效模型关联页缓存", async () => {
    const queryClient = createClient();
    const { assocKey } = seedCaches(queryClient);

    const { result } = renderHook(() => useAllModelsDialog(baseArgs), {
      wrapper: makeWrapper(queryClient),
    });

    await act(async () => {
      await result.current.handleSyncUpstreamModels();
    });

    expect(apiMocks.syncProviderModels).toHaveBeenCalledWith(30);
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
  });

  it("增删自定义模型后失效模型关联页缓存", async () => {
    const queryClient = createClient();
    const { assocKey } = seedCaches(queryClient);
    apiMocks.getProviderModelCatalog.mockResolvedValue([
      { ProviderID: 30, Upstream: [], Custom: ["old-model"] },
    ]);

    const { result } = renderHook(() => useAllModelsDialog(baseArgs), {
      wrapper: makeWrapper(queryClient),
    });

    await act(async () => {
      await result.current.handleAddCustomModels();
    });

    expect(apiMocks.updateProvider).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
  });
});

describe("useProviderMutations", () => {
  beforeEach(() => {
    apiMocks.createProvider.mockReset().mockResolvedValue({});
    apiMocks.updateProvider.mockReset().mockResolvedValue({});
  });

  const formStub = { reset: vi.fn() } as never;
  const formValues = {
    ...defaultProviderFormValues,
    name: "zcode",
    type: "anthropic",
    protocols: ["anthropic"] as const,
  } as unknown as ProviderFormValues;

  function renderMutations(
    queryClient: QueryClient,
    editingProvider: { ID: number } | null,
  ) {
    return renderHook(
      () =>
        useProviderMutations({
          form: formStub,
          editingProvider: editingProvider as never,
          setEditingProvider: vi.fn(),
          setOpen: vi.fn(),
        }),
      { wrapper: makeWrapper(queryClient) },
    );
  }

  it("创建提供商后失效模型关联页缓存（后端 TriggerAutoAssociate 会新增关联）", async () => {
    const queryClient = createClient();
    const { assocKey } = seedCaches(queryClient);

    const { result } = renderMutations(queryClient, null);
    await act(async () => {
      await result.current.handleSubmitProvider(formValues);
    });

    expect(apiMocks.createProvider).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
  });

  it("编辑提供商后失效模型关联页缓存（TriggerAutoAssociate + TriggerAutoClean）", async () => {
    const queryClient = createClient();
    const { assocKey } = seedCaches(queryClient);

    const { result } = renderMutations(queryClient, { ID: 30 });
    await act(async () => {
      await result.current.handleSubmitProvider(formValues);
    });

    expect(apiMocks.updateProvider).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryState(assocKey)?.isInvalidated).toBe(true);
  });
});
