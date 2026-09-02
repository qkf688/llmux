import { act, renderHook } from "@testing-library/react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { beforeEach, describe, expect, it, vi } from "vitest";

// getProvider 需要按请求挂起/放行以模拟竞态，必须在 vi.mock 工厂外持有引用
const { getProviderMock } = vi.hoisted(() => ({ getProviderMock: vi.fn() }));

vi.mock("@/lib/api", () => ({
  getProvider: getProviderMock,
}));

import type { Provider, ProviderDetail } from "@/lib/api";
import { defaultProviderFormValues, providerFormSchema, type ProviderFormValues } from "../form-schema";
import { useProviderDialog } from "./use-provider-dialog";

function createListProvider(id: number, name: string): Provider {
  return { ID: id, Name: name, Type: "openai", Config: "{}", Console: "", Proxy: "" };
}

function createDetail(provider: Provider): ProviderDetail {
  return { ...provider, Endpoints: [], Groups: [] };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

// 组合真实 form hook 与 dialog hook：回填正确性取决于 reset 的实际行为，mock form 会测掉真实语义
function renderDialogHook() {
  const setOpen = vi.fn();
  const setEditingProvider = vi.fn();
  const setDetailLoading = vi.fn();
  const result = renderHook(() => {
    const form = useForm<ProviderFormValues>({
      resolver: zodResolver(providerFormSchema),
      defaultValues: { ...defaultProviderFormValues },
    });
    const dialog = useProviderDialog({ form, setOpen, setEditingProvider, setDetailLoading });
    return { form, ...dialog };
  });
  return { ...result, setOpen, setEditingProvider, setDetailLoading };
}

beforeEach(() => {
  getProviderMock.mockReset();
});

describe("useProviderDialog 编辑弹窗竞态与加载状态", () => {
  it("连点 A→B 且 A 响应晚到时，表单保持 B 的数据（AC-1）", async () => {
    const providerA = createListProvider(1, "A");
    const providerB = createListProvider(2, "B");
    const deferredA = createDeferred<ProviderDetail>();
    const deferredB = createDeferred<ProviderDetail>();
    getProviderMock.mockImplementation((id: number) => (id === 1 ? deferredA.promise : deferredB.promise));

    const hook = renderDialogHook();
    let promiseA!: Promise<void>;
    act(() => {
      // 不 await：让 A 的请求挂在 getProvider(1) 处
      promiseA = hook.result.current.openEditDialog(providerA);
    });

    await act(async () => {
      void hook.result.current.openEditDialog(providerB);
      await deferredB.resolve(createDetail(providerB));
    });

    await act(async () => {
      deferredA.resolve(createDetail(providerA));
      await promiseA;
    });

    // A 晚到的回填必须被丢弃：此时弹窗编辑的是 B
    expect(hook.result.current.form.getValues("name")).toBe("B");
  });

  it("A 请求晚失败时，不关闭 B 的弹窗、不清空 editingProvider（AC-1）", async () => {
    const providerA = createListProvider(1, "A");
    const providerB = createListProvider(2, "B");
    const deferredA = createDeferred<ProviderDetail>();
    const deferredB = createDeferred<ProviderDetail>();
    getProviderMock.mockImplementation((id: number) => (id === 1 ? deferredA.promise : deferredB.promise));

    const hook = renderDialogHook();
    let promiseA!: Promise<void>;
    act(() => {
      promiseA = hook.result.current.openEditDialog(providerA);
    });

    await act(async () => {
      void hook.result.current.openEditDialog(providerB);
      await deferredB.resolve(createDetail(providerB));
    });

    await act(async () => {
      deferredA.reject(new Error("boom"));
      await promiseA;
    });

    // 旧实现：A 的 catch 会 setOpen(false) 把 B 的弹窗关掉
    expect(hook.setOpen).not.toHaveBeenCalledWith(false);
    expect(hook.setEditingProvider).not.toHaveBeenCalledWith(null);
    expect(hook.result.current.form.getValues("name")).toBe("B");
  });

  it("detailLoading 时序：发起时置 true，终态置 false（AC-2/AC-3 状态源）", async () => {
    const providerA = createListProvider(1, "A");
    getProviderMock.mockResolvedValueOnce(createDetail(providerA));

    const hook = renderDialogHook();
    await act(async () => {
      await hook.result.current.openEditDialog(providerA);
    });

    expect(hook.setDetailLoading.mock.calls).toEqual([[true], [false]]);
  });

  it("正常单点：详情回填正确（竞态守卫不得误伤基准路径）", async () => {
    const providerA = createListProvider(1, "A");
    getProviderMock.mockResolvedValueOnce(createDetail(providerA));

    const hook = renderDialogHook();
    await act(async () => {
      await hook.result.current.openEditDialog(providerA);
    });

    expect(hook.result.current.form.getValues("name")).toBe("A");
    expect(hook.setEditingProvider).toHaveBeenCalledWith(providerA);
  });

  it("单点失败：关闭弹窗并复位 editingProvider（现有行为保持）", async () => {
    const providerA = createListProvider(1, "A");
    getProviderMock.mockRejectedValueOnce(new Error("boom"));

    const hook = renderDialogHook();
    await act(async () => {
      await hook.result.current.openEditDialog(providerA);
    });

    expect(hook.setOpen).toHaveBeenCalledWith(false);
    expect(hook.setEditingProvider).toHaveBeenCalledWith(null);
    // 失败路径表单回落为默认值，语义不变
    expect(hook.result.current.form.getValues("name")).toBe("");
  });
});
