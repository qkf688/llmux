import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { getModelProviders, type ModelWithProvider } from "@/lib/api";

import { useModelProvidersAssociationsData } from "./use-model-providers-associations-data";
import { createMockAssociation } from "../test-fixtures";

// 被测链路仅经 useModelProvidersQuery 消费 getModelProviders；
// 另两个为防御性预置：若未来直接 import useModelProvidersAssociationStatus，其网络依赖不会因缺导出而静默 undefined
vi.mock("@/lib/api", () => ({
  getModelProviders: vi.fn(),
  getModelProviderStatus: vi.fn(),
  getModelProviderHealthStatus: vi.fn(),
}));

type LoadProviderStatus = (providers: ModelWithProvider[], modelId: number) => Promise<void>;

type AssociationDataProps = {
  selectedModelId: number | null;
  loadProviderStatus: LoadProviderStatus;
};

function renderAssociationsData(
  selectedModelId: number | null,
  loadProviderStatus: LoadProviderStatus = vi.fn<LoadProviderStatus>(),
) {
  // 每用例独立 QueryClient，避免 react-query 缓存跨用例污染
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const utils = renderHook(
    (props: AssociationDataProps) =>
      useModelProvidersAssociationsData({
        selectedModelId: props.selectedModelId,
        loadProviderStatus: props.loadProviderStatus,
      }),
    { wrapper, initialProps: { selectedModelId, loadProviderStatus } },
  );
  return { ...utils, queryClient, loadProviderStatus };
}

describe("useModelProvidersAssociationsData", () => {
  beforeEach(() => {
    vi.mocked(getModelProviders).mockReset();
  });

  it("loading 期 effect 不随 re-render 反复触发，数据到达后仅同步一次", async () => {
    let resolveData: ((data: ModelWithProvider[]) => void) | undefined;
    vi.mocked(getModelProviders).mockReturnValue(
      new Promise<ModelWithProvider[]>((resolve) => {
        resolveData = resolve;
      }),
    );

    const { rerender, loadProviderStatus } = renderAssociationsData(1);

    // 挂载后 selectedModelId 非空，effect 执行一次
    expect(loadProviderStatus).toHaveBeenCalledTimes(1);

    // 外层状态抖动导致多次 re-render，effect 不得重跑（回归点：稳定空数组引用）。
    // 注：loadProviderStatus 是注入的 vi.fn spy，不触发父组件 setState，故用显式 rerender
    // 模拟外层渲染抖动；真实「effect→setState→re-render」闭环由下方「数据到达后仅同步一次」覆盖
    for (let i = 0; i < 5; i += 1) {
      rerender({ selectedModelId: 1, loadProviderStatus });
    }
    expect(loadProviderStatus).toHaveBeenCalledTimes(1);

    // 数据到达后 effect 再同步一次
    await act(async () => {
      resolveData?.([createMockAssociation(1, 10)]);
    });
    await waitFor(() => expect(loadProviderStatus).toHaveBeenCalledTimes(2));
  });

  it("切换 modelId 后以新数据同步状态", async () => {
    const firstModelData = [createMockAssociation(1, 10)];
    const secondModelData = [createMockAssociation(2, 20)];
    vi.mocked(getModelProviders).mockImplementation(async (modelId: number) =>
      modelId === 1 ? firstModelData : secondModelData,
    );

    const { rerender, loadProviderStatus } = renderAssociationsData(1);
    await waitFor(() => expect(loadProviderStatus).toHaveBeenCalledWith(firstModelData, 1));

    rerender({ selectedModelId: 2, loadProviderStatus });

    // 切换后 hook 先以空数组（新 query loading）同步一次，最终以新数据同步（锁定"先空后新"契约）
    await waitFor(() => expect(loadProviderStatus).toHaveBeenLastCalledWith(secondModelData, 2));
    expect(loadProviderStatus).toHaveBeenCalledWith([], 2);
  });

  it("删除末位关联使列表变空时仍触发状态刷新（避免 providerStatus 残留）", async () => {
    const initialData = [createMockAssociation(1, 10)];
    vi.mocked(getModelProviders).mockResolvedValue(initialData);

    const { result, loadProviderStatus } = renderAssociationsData(1);
    await waitFor(() => expect(loadProviderStatus).toHaveBeenCalledWith(initialData, 1));

    // 模拟删除末位关联：走 hook 公开 API 更新缓存（与生产删除路径一致），不触发 refetch
    act(() => {
      result.current.setModelProviders(() => []);
    });

    await waitFor(() => expect(loadProviderStatus).toHaveBeenLastCalledWith([], 1));
    // 总调用：挂载空数组 + 数据到达 + 列表变空，恰好 3 次
    expect(loadProviderStatus).toHaveBeenCalledTimes(3);
    // 删除走缓存更新，不重新请求网络
    expect(vi.mocked(getModelProviders)).toHaveBeenCalledTimes(1);
  });
});
